package config

import "sort"

func applyBugLevelConfig(cfg *WorkspaceConfig) {
	if cfg == nil {
		return
	}
	if cfg.BugLevel == 0 {
		cfg.BugLevel = 3
	}

	if cfg.ArtifactTypes != nil {
		for _, parentType := range []string{"feature", "task"} {
			if typeCfg := cfg.ArtifactTypes[parentType]; typeCfg != nil {
				typeCfg.AllowedChildren = removeString(typeCfg.AllowedChildren, "bug")
			}
		}
		if _, ok := cfg.ArtifactTypes["bug"]; ok {
			switch cfg.BugLevel {
			case 2:
				if featureCfg := cfg.ArtifactTypes["feature"]; featureCfg != nil {
					featureCfg.AllowedChildren = appendUnique(featureCfg.AllowedChildren, "bug")
				}
			case 3:
				if taskCfg := cfg.ArtifactTypes["task"]; taskCfg != nil {
					taskCfg.AllowedChildren = appendUnique(taskCfg.AllowedChildren, "bug")
				}
			}
		}
	}

	applyBugLevelToQueueLayout(cfg.QueueLayout, cfg.BugLevel)
}

func applyBugLevelToQueueLayout(layout *QueueLayoutConfig, bugLevel int) {
	if layout == nil {
		return
	}

	targetLevel := bugLevel
	if targetLevel == 0 {
		targetLevel = 3
	}

	foundTarget := false
	for i := range layout.Levels {
		layout.Levels[i].Types = removeString(layout.Levels[i].Types, "bug")
		if layout.Levels[i].Level == targetLevel {
			layout.Levels[i].Types = appendUnique(layout.Levels[i].Types, "bug")
			foundTarget = true
		}
	}

	if !foundTarget {
		layout.Levels = append(layout.Levels, HierarchyLevel{
			Level: targetLevel,
			Types: []string{"bug"},
		})
		sort.SliceStable(layout.Levels, func(i, j int) bool {
			return layout.Levels[i].Level < layout.Levels[j].Level
		})
	}
}

func appendUnique(values []string, target string) []string {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

func removeString(values []string, target string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
