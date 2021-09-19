package colortext

type Color byte

const (
	None  Color = 0
	White Color = iota + 0x80
	Purple
	Yellow
	Green
	Blue
	Red
	Gray
	Orange
	Cyan
	Lavender
	Gold
	Lime
	Steel
	Pink
	Brown
	Peach
)

var colorTags = map[string]Color{
	"white":    White,
	"purple":   Purple,
	"yellow":   Yellow,
	"green":    Green,
	"blue":     Blue,
	"red":      Red,
	"gray":     Gray,
	"orange":   Orange,
	"cyan":     Cyan,
	"lavender": Lavender,
	"gold":     Gold,
	"lime":     Lime,
	"steel":    Steel,
	"pink":     Pink,
	"brown":    Brown,
	"peach":    Peach,
}
