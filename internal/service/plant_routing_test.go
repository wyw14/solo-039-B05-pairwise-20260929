package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/solo-039/internal/model"
)

func newStartedPlant(t *testing.T) *Plant {
	t.Helper()
	plant := NewPlant(t.TempDir())
	if _, err := plant.Start(context.Background(), "baseline", "primary"); err != nil {
		t.Fatalf("start plant: %v", err)
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("unexpected phase after start: %s", phase)
	}
	return plant
}

func addSamples(t *testing.T, plant *Plant, values ...float64) {
	t.Helper()
	base := time.Now().Unix()
	for i, value := range values {
		if _, err := plant.AddSample(value, base+int64(i)); err != nil {
			t.Fatalf("add sample %d: %v", i, err)
		}
	}
}

// 样本数量凑够但数据不稳定时，稳定阶段的批次不得被放行，更不能被改判为达标。
func TestRouteProductRejectsUnstableSamplesDuringStabilizing(t *testing.T) {
	plant := newStartedPlant(t)
	addSamples(t, plant, 0.10, 0.50, 0.90)

	err := plant.RouteProduct("tank-A")
	if err == nil {
		t.Fatal("release succeeded on unstable samples")
	}
	if !strings.Contains(err.Error(), "unstable") {
		t.Fatalf("expected instability rejection, got: %v", err)
	}

	current := plant.Campaign.Snapshot()
	if current.Phase != model.Stabilizing {
		t.Fatalf("phase changed to %s despite failed stability confirmation", current.Phase)
	}
	if targets := plant.ProductTargets(); len(targets) != 0 {
		t.Fatalf("product route opened without stability confirmation: %v", targets)
	}
}

// 样本不足时放行同样必须被拒绝。
func TestRouteProductRejectsInsufficientSamplesDuringStabilizing(t *testing.T) {
	plant := newStartedPlant(t)
	addSamples(t, plant, 0.50, 0.51)

	if err := plant.RouteProduct("tank-A"); err == nil {
		t.Fatal("release succeeded before enough samples were collected")
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("phase changed to %s despite insufficient evidence", phase)
	}
	if targets := plant.ProductTargets(); len(targets) != 0 {
		t.Fatalf("product route opened with insufficient evidence: %v", targets)
	}
}

// 真正稳定达标后，批次经合法迁移进入 on-spec，正常产品路由仍可执行。
func TestRouteProductSucceedsAfterStabilityConfirmed(t *testing.T) {
	plant := newStartedPlant(t)
	addSamples(t, plant, 0.50, 0.51, 0.50)

	if err := plant.RouteProduct("tank-A"); err != nil {
		t.Fatalf("release failed after stability confirmation: %v", err)
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.OnSpec {
		t.Fatalf("expected on-spec after release, got %s", phase)
	}
	if targets := plant.ProductTargets(); len(targets) != 1 || targets[0] != "tank-A" {
		t.Fatalf("product route not opened after legitimate release: %v", targets)
	}

	// 已达标批次的后续正常产品路由仍可执行。
	if err := plant.RouteProduct("tank-B"); err != nil {
		t.Fatalf("normal routing for on-spec campaign failed: %v", err)
	}
	if targets := plant.ProductTargets(); len(targets) != 1 || targets[0] != "tank-B" {
		t.Fatalf("product route not switched on re-route: %v", targets)
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.OnSpec {
		t.Fatalf("phase changed away from on-spec: %s", phase)
	}
}
