package db

import "github.com/softwaresalt/backlogit/internal/models"

// IsTaskRouteEligible reports whether a task carries complete, valid size and
// complexity provenance (red-phase stub).
func IsTaskRouteEligible(_ *models.Artifact) bool { return false }
