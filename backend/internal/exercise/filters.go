package exercise

// MuscleCategory groups the detailed muscle names stored in the catalog under
// the broad label that the exercise selector offers first.
type MuscleCategory struct {
	Name    string   `json:"name"`
	Muscles []string `json:"muscles"`
}

// FilterOptions lists the equipment and muscle values that clients may send
// to Search. Search compares them for equality, so every value must be
// spelled exactly as in the exercise catalog; the catalog test in the main
// package fails when the two drift apart.
type FilterOptions struct {
	Equipment        []string         `json:"equipment"`
	MuscleCategories []MuscleCategory `json:"muscle_categories"`
}

// AvailableFilterOptions is the single definition of the filter choices shown
// by the exercise dictionary, the exercise selector and the equipment
// preferences. It returns a fresh value on every call so that callers cannot
// modify the shared definition.
func AvailableFilterOptions() FilterOptions {
	return FilterOptions{
		Equipment: []string{
			"自重", "ダンベル", "バーベル", "EZバー", "マシン", "ケーブル",
			"チューブ・バンド", "ケトルベル", "メディシンボール", "バランスボール", "フォームローラー", "その他",
		},
		MuscleCategories: []MuscleCategory{
			{Name: "胸", Muscles: []string{"大胸筋"}},
			{Name: "背中", Muscles: []string{"広背筋", "背中中部", "下背部", "僧帽筋"}},
			{Name: "肩", Muscles: []string{"肩"}},
			{Name: "腕", Muscles: []string{"上腕二頭筋", "上腕三頭筋", "前腕"}},
			{Name: "脚", Muscles: []string{"大腿四頭筋", "ハムストリングス", "大臀筋", "ふくらはぎ", "内転筋", "外転筋"}},
			{Name: "腹/体幹", Muscles: []string{"腹筋"}},
			{Name: "その他", Muscles: []string{"首"}},
		},
	}
}
