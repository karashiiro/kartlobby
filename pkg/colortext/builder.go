package colortext

type ColorTextBuilder struct {
	buffer []byte
}

func New() *ColorTextBuilder {
	return &ColorTextBuilder{}
}

func (b *ColorTextBuilder) AppendText(text string) *ColorTextBuilder {
	b.buffer = append(b.buffer, []byte(text)...)
	return b
}

func (b *ColorTextBuilder) AppendTextColored(text string, color Color) *ColorTextBuilder {
	b.buffer = append(b.buffer, []byte{byte(color)}...)
	b.buffer = append(b.buffer, []byte(text)...)
	return b
}

func (b *ColorTextBuilder) Build() string {
	return string(b.buffer)
}
