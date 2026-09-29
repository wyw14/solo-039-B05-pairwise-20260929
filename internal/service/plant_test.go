package service

import (
	"context"
	"testing"

	"github.com/wyw14/solo-039/internal/model"
)

func startStabilizing(t *testing.T) *Plant {
	t.Helper()
	plant := NewPlant(t.TempDir())
	if _, err := plant.Start(context.Background(), "baseline", "primary"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("expected stabilizing after start, got %s", phase)
	}
	return plant
}

func addStableSamples(t *testing.T, plant *Plant) {
	t.Helper()
	for i, value := range []float64{1.00, 1.01, 0.99} {
		if _, err := plant.AddSample(value, int64(i+1)); err != nil {
			t.Fatalf("add sample %d: %v", i, err)
		}
	}
}

func TestRouteProductRejectedWhileStabilizing(t *testing.T) {
	plant := startStabilizing(t)
	addStableSamples(t, plant)

	if err := plant.RouteProduct("truck"); err == nil {
		t.Fatal("expected routing to be rejected while campaign is stabilizing")
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("routing request must not change campaign phase, got %s", phase)
	}
	if plant.Product.Route.Opened() {
		t.Fatal("route must stay closed while campaign is stabilizing")
	}
}

func TestRouteProductAllowedAfterConfirmSpec(t *testing.T) {
	plant := startStabilizing(t)
	addStableSamples(t, plant)
	epochBefore := plant.Campaign.Snapshot().Epoch

	confirmed, err := plant.ConfirmSpec()
	if err != nil {
		t.Fatalf("confirm spec: %v", err)
	}
	if confirmed.Phase != model.OnSpec {
		t.Fatalf("expected on-spec after confirmation, got %s", confirmed.Phase)
	}
	if confirmed.Epoch <= epochBefore {
		t.Fatalf("expected epoch to advance on confirmation, got %d before %d", confirmed.Epoch, epochBefore)
	}
	if err := plant.RouteProduct("truck"); err != nil {
		t.Fatalf("routing after confirmation: %v", err)
	}
	if !plant.Product.Route.Opened() {
		t.Fatal("expected route to be open after confirmed on-spec")
	}
	if got := plant.ProductTargets(); len(got) != 1 || got[0] != "truck" {
		t.Fatalf("expected product target truck, got %v", got)
	}
}

func TestConfirmSpecRequiresStableEvidence(t *testing.T) {
	plant := startStabilizing(t)

	if _, err := plant.ConfirmSpec(); err == nil {
		t.Fatal("expected confirmation to fail without enough samples")
	}

	for i, value := range []float64{1.0, 1.5, 0.5} {
		if _, err := plant.AddSample(value, int64(i+1)); err != nil {
			t.Fatalf("add sample %d: %v", i, err)
		}
	}
	if _, err := plant.ConfirmSpec(); err == nil {
		t.Fatal("expected confirmation to fail with unstable samples")
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("failed confirmation must not change campaign phase, got %s", phase)
	}
}

func TestTransitionReturnsToStabilizing(t *testing.T) {
	plant := startStabilizing(t)
	addStableSamples(t, plant)
	if _, err := plant.ConfirmSpec(); err != nil {
		t.Fatalf("confirm spec: %v", err)
	}
	if _, err := plant.Transition(context.Background(), "premium", "secondary"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if phase := plant.Campaign.Snapshot().Phase; phase != model.Stabilizing {
		t.Fatalf("expected stabilizing after grade transition, got %s", phase)
	}
	if err := plant.RouteProduct("truck"); err == nil {
		t.Fatal("expected routing to be rejected for unconfirmed new grade")
	}
}
