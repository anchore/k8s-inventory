package table

import (
	"io"
	"sort"

	"github.com/anchore/kai/kai/result"

	"github.com/olekukonko/tablewriter"
)

// Presenter is a generic struct for holding fields needed for reporting
type Presenter struct {
	result result.Result
}

// NewPresenter is a *Presenter constructor
func NewPresenter(result result.Result) *Presenter {
	return &Presenter{
		result: result,
	}
}

// Present creates a JSON-based reporting
func (pres *Presenter) Present(output io.Writer) error {
	rows := make([][]string, 0)

	columns := []string{"Namespace", "Image"}
	for _, n := range pres.result.Results {
		namespace := n.Namespace
		for _, image := range n.Images {
			row := []string{namespace, image}
			rows = append(rows, row)
		}
	}

	if len(rows) == 0 {
		_, err := io.WriteString(output, "No Images found\n")
		return err
	}

	// sort by name, version, then type
	sort.SliceStable(rows, func(i, j int) bool {
		for col := 0; col < len(columns); col++ {
			if rows[i][0] != rows[j][0] {
				return rows[i][col] < rows[j][col]
			}
		}
		return false
	})

	table := tablewriter.NewWriter(output)

	table.SetHeader(columns)
	table.SetAutoWrapText(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetAutoFormatHeaders(true)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	table.AppendBulk(rows)
	table.Render()

	return nil
}
