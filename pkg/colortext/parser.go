package colortext

// Parse parses colored text from a string. The input is expected to be a normal string with
// colored portions prefixed with "<color>", replacing "color" with the desired actual color.
func Parse(text string) string {
	b := New()

	readingColor := false
	currentColorStr := ""
	currentColor := None
	for _, c := range text {
		if c == '<' {
			// Tag open
			readingColor = true
			continue
		} else if c == '>' {
			// Tag close
			readingColor = false

			// Parse the color tag
			if color, ok := colorTags[currentColorStr]; ok {
				currentColor = color
			}

			// Reset the color string
			currentColorStr = ""

			continue
		}

		if readingColor {
			currentColorStr += string(c)
		} else {
			if currentColor == None {
				b.AppendText(string(c))
			} else {
				b.AppendTextColored(string(c), currentColor)

				// Reset the color since it'll persist until we change it again
				currentColor = None
			}
		}
	}

	return b.Build()
}
