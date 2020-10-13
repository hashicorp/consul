package logging

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

func TestColorWriter(t *testing.T) {
	patchNoColor(t)

	t.Run("no label match", func(t *testing.T) {
		buf := new(bytes.Buffer)
		w := &colorWriter{out: buf}

		log := "2017-07-17 [INFO]  Name: this is some text"
		n, err := fmt.Fprint(w, log)
		require.NoError(t, err)
		require.Equal(t, len(log), n)
		require.Equal(t, log, buf.String())
	})

	t.Run("with warn label", func(t *testing.T) {
		buf := new(bytes.Buffer)
		w := &colorWriter{out: buf}

		log := "2017-07-17 [WARN]  Name: this is some text"
		n, err := fmt.Fprint(w, log)
		require.NoError(t, err)
		require.Equal(t, len(log), n)
		expected := "2017-07-17 " + color.YellowString("[WARN]") + "  Name: this is some text"
		require.Equal(t, expected, buf.String())
	})

	t.Run("with error label", func(t *testing.T) {
		buf := new(bytes.Buffer)
		w := &colorWriter{out: buf}

		log := "2017-07-17 [ERROR] Name: this is some text"
		n, err := fmt.Fprint(w, log)
		require.NoError(t, err)
		require.Equal(t, len(log), n)
		expected := "2017-07-17 " + color.RedString("[ERROR]") + " Name: this is some text"
		require.Equal(t, expected, buf.String())
	})
}

func patchNoColor(t *testing.T) {
	var orig bool
	orig, color.NoColor = color.NoColor, false
	t.Cleanup(func() {
		color.NoColor = orig
	})
}
