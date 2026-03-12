package model

import (
	"math"
	"sort"
	"time"
)

type UserWeeklyUsage struct {
	UserId       int      `json:"user_id"`
	Username     string   `json:"username"`
	WeeklyQuota  int64    `json:"weekly_quota"`
	RequestCount int64    `json:"request_count"`
	TopModels    []string `json:"top_models"`
}

type userAgg struct {
	UserId       int    `gorm:"column:user_id"`
	Username     string `gorm:"column:username"`
	WeeklyQuota  int64  `gorm:"column:weekly_quota"`
	RequestCount int64  `gorm:"column:request_count"`
}

type modelAgg struct {
	UserId     int    `gorm:"column:user_id"`
	ModelName  string `gorm:"column:model_name"`
	ModelQuota int64  `gorm:"column:model_quota"`
}

func GetUserWeeklyUsage(group string) ([]UserWeeklyUsage, error) {
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	// Step 1: aggregate per-user quota and request count
	var aggs []userAgg
	tx := LOG_DB.Table("logs").
		Select("user_id, username, SUM(quota) as weekly_quota, COUNT(*) as request_count").
		Where("type = ? AND created_at >= ?", LogTypeConsume, sevenDaysAgo)

	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
	}

	err := tx.Group("user_id, username").
		Having("SUM(quota) > 0").
		Find(&aggs).Error
	if err != nil {
		return nil, err
	}

	if len(aggs) == 0 {
		return []UserWeeklyUsage{}, nil
	}

	// Step 2: get top models per user
	userIds := make([]int, len(aggs))
	for i, a := range aggs {
		userIds[i] = a.UserId
	}

	var modelAggs []modelAgg
	err = LOG_DB.Table("logs").
		Select("user_id, model_name, SUM(quota) as model_quota").
		Where("type = ? AND created_at >= ? AND user_id IN ?", LogTypeConsume, sevenDaysAgo, userIds).
		Group("user_id, model_name").
		Find(&modelAggs).Error
	if err != nil {
		return nil, err
	}

	// Group by user_id, sort by model_quota desc, keep top 3
	userModelsMap := make(map[int][]modelAgg)
	for _, m := range modelAggs {
		userModelsMap[m.UserId] = append(userModelsMap[m.UserId], m)
	}
	for uid := range userModelsMap {
		sort.Slice(userModelsMap[uid], func(i, j int) bool {
			return userModelsMap[uid][i].ModelQuota > userModelsMap[uid][j].ModelQuota
		})
	}

	// Build result
	results := make([]UserWeeklyUsage, len(aggs))
	for i, a := range aggs {
		var topModels []string
		models := userModelsMap[a.UserId]
		for j := 0; j < len(models) && j < 3; j++ {
			if models[j].ModelName != "" {
				topModels = append(topModels, models[j].ModelName)
			}
		}
		results[i] = UserWeeklyUsage{
			UserId:       a.UserId,
			Username:     a.Username,
			WeeklyQuota:  a.WeeklyQuota,
			RequestCount: a.RequestCount,
			TopModels:    topModels,
		}
	}

	return results, nil
}

// SelectOptimalUsers finds the subset of maxCount users whose combined weekly
// quota is closest to budgetQuota. Prefers combinations that do not exceed the budget.
func SelectOptimalUsers(users []UserWeeklyUsage, maxCount int, budgetQuota int64) []UserWeeklyUsage {
	if len(users) == 0 {
		return []UserWeeklyUsage{}
	}
	if maxCount >= len(users) {
		return users
	}
	if maxCount <= 0 {
		return []UserWeeklyUsage{}
	}

	// Sort by WeeklyQuota descending for deterministic results
	sorted := make([]UserWeeklyUsage, len(users))
	copy(sorted, users)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].WeeklyQuota > sorted[j].WeeklyQuota
	})

	n := len(sorted)

	// Use enumeration for small combinations, otherwise greedy
	if maxCount <= 8 && combinationCount(n, maxCount) <= 100000 {
		return enumerateBestCombination(sorted, maxCount, budgetQuota)
	}
	return greedySelect(sorted, maxCount, budgetQuota)
}

func enumerateBestCombination(users []UserWeeklyUsage, k int, budget int64) []UserWeeklyUsage {
	n := len(users)
	indices := make([]int, k)
	for i := range indices {
		indices[i] = i
	}

	var bestIndices []int
	var bestDiff int64 = math.MaxInt64
	bestUnder := false

	for {
		// Compute sum for current combination
		var total int64
		for _, idx := range indices {
			total += users[idx].WeeklyQuota
		}
		diff := abs64(total - budget)
		under := total <= budget

		// Prefer under-budget; among same category, prefer smaller diff
		if bestIndices == nil ||
			(under && !bestUnder) ||
			(under == bestUnder && diff < bestDiff) {
			bestIndices = make([]int, k)
			copy(bestIndices, indices)
			bestDiff = diff
			bestUnder = under
		}

		// Generate next combination
		if !nextCombination(indices, n) {
			break
		}
	}

	result := make([]UserWeeklyUsage, k)
	for i, idx := range bestIndices {
		result[i] = users[idx]
	}
	return result
}

func greedySelect(users []UserWeeklyUsage, maxCount int, budget int64) []UserWeeklyUsage {
	// users are sorted descending by WeeklyQuota
	selected := make([]UserWeeklyUsage, 0, maxCount)
	used := make(map[int]bool)
	var remaining = budget

	// Pick users that fit under remaining budget
	for i := range users {
		if len(selected) >= maxCount {
			break
		}
		if users[i].WeeklyQuota <= remaining {
			selected = append(selected, users[i])
			used[users[i].UserId] = true
			remaining -= users[i].WeeklyQuota
		}
	}

	// If not enough, fill from smallest unused users
	if len(selected) < maxCount {
		for i := len(users) - 1; i >= 0; i-- {
			if len(selected) >= maxCount {
				break
			}
			if !used[users[i].UserId] {
				selected = append(selected, users[i])
				used[users[i].UserId] = true
			}
		}
	}

	return selected
}

// nextCombination advances indices to the next combination in lexicographic order.
// Returns false when all combinations have been exhausted.
func nextCombination(indices []int, n int) bool {
	k := len(indices)
	for i := k - 1; i >= 0; i-- {
		if indices[i] < n-k+i {
			indices[i]++
			for j := i + 1; j < k; j++ {
				indices[j] = indices[j-1] + 1
			}
			return true
		}
	}
	return false
}

func combinationCount(n, k int) int64 {
	if k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n-k {
		k = n - k
	}
	var result int64 = 1
	for i := 0; i < k; i++ {
		result = result * int64(n-i) / int64(i+1)
		if result > 100000 {
			return result // early exit, already exceeds threshold
		}
	}
	return result
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
