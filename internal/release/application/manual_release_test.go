package application

import (
	"errors"
	"net/http"
	"testing"

	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

func TestCreateManualQueuesEachRequestWithoutDispatch(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	service := &model.Service{
		ID:              uuid.New(),
		ServiceKey:      "svc-manual",
		AgentID:         "offline-agent",
		ImageRepo:       "registry.example.com/team/service",
		CurrentLiveSlot: model.SlotBlue,
		Enabled:         boolPointer(true),
	}
	if err := serviceRepo.Create(service); err != nil {
		t.Fatal(err)
	}
	svc := NewService(releaseRepo, dispatcher, servicecatalogapp.NewService(serviceRepo), nil)
	ids := make(map[uuid.UUID]bool)
	for _, tag := range []string{"latest", "latest", "v1.2.3"} {
		output, err := svc.CreateManual(dto.CreateManualReleaseRequest{
			ServiceID:    service.ID,
			ImageTag:     tag,
			ReleaseNotes: "manual deployment",
		}, "admin")
		if err != nil {
			t.Fatalf("CreateManual(%q) error = %v", tag, err)
		}
		if output.ID == uuid.Nil || ids[output.ID] {
			t.Fatalf("each request must create a new release, got %s", output.ID)
		}
		ids[output.ID] = true
		if output.ServiceID != service.ID || output.AgentID != service.AgentID || output.ImageRepo != service.ImageRepo || output.ImageTag != tag {
			t.Fatalf("unexpected service/image snapshot: %#v", output)
		}
		if output.TriggeredBy != "admin" || output.ReleaseNotes != "manual deployment" {
			t.Fatalf("unexpected manual release metadata: %#v", output)
		}
		if output.Status != model.ReleaseStatusQueued || output.CurrentTaskID != nil || output.TrafficPercent != 0 {
			t.Fatalf("expected queued release without task: %#v", output)
		}
		if output.TargetSlot != model.SlotGreen || output.PreviousLiveSlot != model.SlotBlue || output.SwitchConfirmed == nil || *output.SwitchConfirmed {
			t.Fatalf("unexpected queued deployment state: %#v", output)
		}
	}
	if len(releaseRepo.releases) != 3 || len(releaseRepo.audits) != 3 {
		t.Fatalf("expected three releases and audits, got releases=%d audits=%d", len(releaseRepo.releases), len(releaseRepo.audits))
	}
	for _, audit := range releaseRepo.audits {
		if audit.EventType != "release_requested" || !ids[uuid.MustParse(audit.AggregateID)] {
			t.Fatalf("unexpected creation audit: %#v", audit)
		}
	}
	if len(releaseRepo.tasks) != 0 || len(dispatcher.tasks) != 0 {
		t.Fatalf("manual creation dispatched work: persisted=%d dispatched=%d", len(releaseRepo.tasks), len(dispatcher.tasks))
	}
}

func TestCreateManualRejectsInvalidRequestsWithoutWrites(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enabled    bool
		missing    bool
		tag        string
		statusCode int
	}{
		{name: "service not found", enabled: true, missing: true, tag: "latest", statusCode: http.StatusNotFound},
		{name: "service disabled", tag: "latest", statusCode: http.StatusBadRequest},
		{name: "invalid tag", enabled: true, tag: "repo:latest", statusCode: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceRepo := &fakeServiceRepo{}
			releaseRepo := newFakeReleaseRepo()
			dispatcher := &fakeDispatcher{}
			service := &model.Service{
				ID:         uuid.New(),
				ServiceKey: "svc-manual",
				AgentID:    "offline-agent",
				ImageRepo:  "registry.example.com/team/service",
				Enabled:    boolPointer(tc.enabled),
			}
			if !tc.missing {
				if err := serviceRepo.Create(service); err != nil {
					t.Fatal(err)
				}
			}
			svc := NewService(releaseRepo, dispatcher, servicecatalogapp.NewService(serviceRepo), nil)
			_, err := svc.CreateManual(dto.CreateManualReleaseRequest{ServiceID: service.ID, ImageTag: tc.tag}, "admin")
			var statusCarrier interface{ GetStatusCode() int }
			if !errors.As(err, &statusCarrier) || statusCarrier.GetStatusCode() != tc.statusCode {
				t.Fatalf("expected status %d, got %v", tc.statusCode, err)
			}
			if len(releaseRepo.releases) != 0 || len(releaseRepo.audits) != 0 || len(releaseRepo.tasks) != 0 || len(dispatcher.tasks) != 0 {
				t.Fatal("rejected request must not persist or dispatch work")
			}
		})
	}
}
