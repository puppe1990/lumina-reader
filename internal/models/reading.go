package models

import "time"

const ReadingGoalMinutes = 30

// Streak conta dias consecutivos de leitura terminando hoje — ou ontem, quando
// hoje ainda não teve progresso (a sequência não zera antes do fim do dia).
// days vem em ordem decrescente ("2006-01-02").
func Streak(days []string, today string) int {
	cursor, err := time.Parse("2006-01-02", today)
	if err != nil {
		return 0
	}
	streak := 0
	for _, day := range days {
		esperado := cursor.Format("2006-01-02")
		if day == esperado {
			streak++
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		if streak == 0 && day == cursor.AddDate(0, 0, -1).Format("2006-01-02") {
			streak++
			cursor = cursor.AddDate(0, 0, -2)
			continue
		}
		break
	}
	return streak
}
