package ui

import (
	"github.com/anchore/kai/internal/ui/common"
	"github.com/gookit/color"
	"github.com/wagoodman/go-progress/format"
	"github.com/wagoodman/jotframe/pkg/frame"
)

const maxBarWidth = 50
const statusSet = common.SpinnerDotSet // SpinnerCircleOutlineSet
const completedStatus = "✔"            // "●"
const tileFormat = color.Bold
const statusTitleTemplate = " %s %-28s "

var auxInfoFormat = color.HEX("#777777")

func startProcess() (format.Simple, *common.Spinner) {
	width, _ := frame.GetTerminalSize()
	barWidth := int(0.25 * float64(width))
	if barWidth > maxBarWidth {
		barWidth = maxBarWidth
	}
	formatter := format.NewSimpleWithTheme(barWidth, format.HeavyNoBarTheme, format.ColorCompleted, format.ColorTodo)
	spinner := common.NewSpinner(statusSet)

	return formatter, &spinner
}
