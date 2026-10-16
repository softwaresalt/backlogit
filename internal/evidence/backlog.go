package evidence

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/parser"
)

// Backlog is the read-only backlogit workspace evidence source.
type Backlog struct {
	// Root is the backlog workspace directory containing queue/ and archive/.
	Root string
}

// Member is one shipment member as recorded in the workspace.
type Member struct {
	ID           string
	ArtifactType string
	// Status is the effective status; an archived item reports the status it
	// held when archived.
	Status   string
	Archived bool
	Commit   string
}

// Provenance records commit and archive provenance for a shipment.
type Provenance struct {
	ShipmentCommit string
	MemberCommits  map[string]string
	ArchivedFrom   string
}

// BacklogResult is the derived shipment/task/provenance evidence.
type BacklogResult struct {
	Status         Status
	Reason         string
	ShipmentID     string
	ShipmentStatus string
	// Closed is true when the shipment's effective status is shipped.
	Closed     bool
	Members    []Member
	TaskTotal  int
	TasksDone  int
	TaskCounts map[string]int
	Provenance Provenance
}

var itemIDPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*-[A-Z]+$`)

// doneStatuses are effective statuses that count a task as complete.
var doneStatuses = map[string]bool{"done": true, "shipped": true, "accepted": true}

// load reads one item from queue/ then archive/, never leaving Root.
func (b *Backlog) load(id string) (*models.Artifact, error) {
	if !itemIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid item id %q", id)
	}
	for _, dir := range []string{"queue", "archive"} {
		a, _, err := parser.ParseMarkdownFile(filepath.Join(b.Root, dir, id+".md"))
		if err == nil {
			if a.ID != id {
				return nil, fmt.Errorf("load %s: file declares id %q", id, a.ID)
			}
			return a, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("load %s: %w", id, err)
		}
	}
	return nil, fmt.Errorf("item %s not found in queue or archive", id)
}

func effectiveStatus(a *models.Artifact) string {
	if a.ArchivedStatus != "" && string(a.Status) == "archived" {
		return a.ArchivedStatus
	}
	return string(a.Status)
}

// ShipmentEvidence derives shipment membership, status, task counts,
// provenance and closure state for a shipment. Any unreadable or missing
// source yields StatusDegraded with no partial evidence.
func (b *Backlog) ShipmentEvidence(shipmentID string) BacklogResult {
	fail := func(format string, args ...any) BacklogResult {
		return BacklogResult{Status: StatusDegraded, ShipmentID: shipmentID, Reason: fmt.Sprintf(format, args...)}
	}
	ship, err := b.load(shipmentID)
	if err != nil {
		return fail("shipment: %v", err)
	}
	if ship.ArtifactType != "shipment" {
		return fail("%s is not a shipment (artifact_type %q)", shipmentID, ship.ArtifactType)
	}

	res := BacklogResult{
		Status:         StatusVerified,
		ShipmentID:     shipmentID,
		ShipmentStatus: effectiveStatus(ship),
		TaskCounts:     map[string]int{},
		Provenance: Provenance{
			ShipmentCommit: ship.Commit,
			ArchivedFrom:   ship.ArchivedFrom,
			MemberCommits:  map[string]string{},
		},
	}
	res.Closed = res.ShipmentStatus == "shipped"

	var missing []string
	for _, id := range core.NormalizeShipmentItems(ship) {
		a, err := b.load(id)
		if err != nil {
			missing = append(missing, id)
			continue
		}
		m := Member{
			ID:           a.ID,
			ArtifactType: a.ArtifactType,
			Status:       effectiveStatus(a),
			Archived:     string(a.Status) == "archived",
			Commit:       a.Commit,
		}
		res.Members = append(res.Members, m)
		if m.Commit != "" {
			res.Provenance.MemberCommits[m.ID] = m.Commit
		}
		if m.ArtifactType == "task" {
			res.TaskTotal++
			res.TaskCounts[m.Status]++
			if doneStatuses[m.Status] {
				res.TasksDone++
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fail("shipment members unverifiable: %s", strings.Join(missing, ", "))
	}
	return res
}
