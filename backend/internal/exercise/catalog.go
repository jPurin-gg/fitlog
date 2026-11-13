package exercise

import (
	"strings"
	"unicode"
)

// DefaultCategory は分類が未指定のときに保存する分類。
const DefaultCategory = "筋力トレーニング"

// Categories は保存できる分類。カタログデータ（tmpkin_jp.json）と同じ日本語ラベルに揃える。
var Categories = []string{
	"筋力トレーニング",
	"ストレッチ",
	"プライオメトリクス",
	"パワーリフティング",
	"ウエイトリフティング",
	"ストロングマン",
	"有酸素運動",
}

// legacyCategoryCodes は以前の辞書ページなどが送っていた英語コードをカタログ表記へ寄せる。
var legacyCategoryCodes = map[string]string{
	"strength":              "筋力トレーニング",
	"stretching":            "ストレッチ",
	"plyometrics":           "プライオメトリクス",
	"powerlifting":          "パワーリフティング",
	"olympic_weightlifting": "ウエイトリフティング",
	"strongman":             "ストロングマン",
	"cardio":                "有酸素運動",
}

// Muscles はメイン部位として保存できる部位。カタログデータの部位と同じ表記に揃える。
// カタログにない部位で保存すると、部位検索・プラン候補・代替相談の候補に一致しなくなる。
var Muscles = []string{
	"大胸筋",
	"背中中部",
	"広背筋",
	"下背部",
	"肩",
	"上腕二頭筋",
	"上腕三頭筋",
	"前腕",
	"腹筋",
	"大臀筋",
	"大腿四頭筋",
	"ハムストリングス",
	"ふくらはぎ",
	"首",
	"僧帽筋",
	"内転筋",
	"外転筋",
}

// NormalizeCategory は分類をカタログ表記へ変換する。空なら DefaultCategory を返す。
// 既知の表記でなければ ok=false を返す。
func NormalizeCategory(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultCategory, true
	}
	if label, exists := legacyCategoryCodes[strings.ToLower(value)]; exists {
		return label, true
	}
	for _, category := range Categories {
		if value == category {
			return category, true
		}
	}
	return "", false
}

func isKnownMuscle(value string) bool {
	for _, muscle := range Muscles {
		if value == muscle {
			return true
		}
	}
	return false
}

// NormalizeName は同名判定に使うキーを返す。
//   - 前後の空白を除く
//   - 全角英数字・記号（U+FF01〜U+FF5E）を半角にする
//   - 空白（全角スペースを含む）をすべて除く
//   - 英字の大文字・小文字を区別しない
func NormalizeName(name string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if r >= '！' && r <= '～' {
			r -= '！' - '!'
		}
		if unicode.IsSpace(r) {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}
