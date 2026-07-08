package context

import "fmt"

func ManifestBudget(manifest Manifest) int {
	total := 0
	total += EstimateNoLLM(manifest.Slug)
	total += EstimateNoLLM(manifest.TaskID)
	total += EstimateNoLLM(manifest.Mode)
	for _, item := range manifest.Items {
		total += EstimateNoLLM(item.Kind)
		total += EstimateNoLLM(item.Path)
		total += EstimateNoLLM(item.TaskID)
		total += item.EstimatedTokens
	}
	return total
}

func CheckBudget(manifest Manifest, maxTokens int) error {
	if maxTokens <= 0 {
		return nil
	}
	used := ManifestBudget(manifest)
	if used > maxTokens {
		return fmt.Errorf("context budget exceeded: %d > %d", used, maxTokens)
	}
	return nil
}
