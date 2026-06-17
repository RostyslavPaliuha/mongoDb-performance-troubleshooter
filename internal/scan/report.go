package scan

import (
	"bytes"
	"encoding/json"
	"html/template"
	"time"
)

func RenderHTML(report Report) (string, error) {
	var buffer bytes.Buffer
	if err := reportTemplate.Execute(&buffer, reportView{
		Report:        report,
		FindingCount:  len(report.Findings),
		FormattedFrom: report.StartedAt.Format(time.RFC3339),
		FormattedTo:   report.EndedAt.Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

type reportView struct {
	Report        Report
	FindingCount  int
	FormattedFrom string
	FormattedTo   string
}

func prettyCommand(command map[string]any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(command); err != nil {
		return "<unrenderable command>"
	}
	return buffer.String()
}

func runtimeMS(operation Operation) string {
	return (time.Duration(operation.MicrosecsRunning) * time.Microsecond).String()
}

var reportTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"prettyCommand": prettyCommand,
	"runtimeMS":     runtimeMS,
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>MongoDB Performance Troubleshooter Report</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; color: #172026; background: #f7f9fb; }
    main { max-width: 1120px; margin: 0 auto; }
    h1, h2, h3 { color: #102a43; }
    .summary, .finding { background: #fff; border: 1px solid #d9e2ec; border-radius: 8px; padding: 20px; margin: 16px 0; }
    .metric { display: inline-block; margin-right: 24px; }
    .label { color: #627d98; font-size: 13px; text-transform: uppercase; letter-spacing: .04em; }
    pre { background: #102a43; color: #f0f4f8; padding: 16px; overflow-x: auto; border-radius: 6px; }
    table { border-collapse: collapse; width: 100%; margin: 12px 0; }
    th, td { border-bottom: 1px solid #d9e2ec; padding: 8px; text-align: left; }
    .warning { color: #9f580a; }
  </style>
</head>
<body>
<main>
  <h1>MongoDB Performance Troubleshooter Report</h1>
  <section class="summary">
    <div class="metric"><div class="label">Started</div><div>{{.FormattedFrom}}</div></div>
    <div class="metric"><div class="label">Ended</div><div>{{.FormattedTo}}</div></div>
    <div class="metric"><div class="label">Duration</div><div>{{.Report.Duration}}</div></div>
    <div class="metric"><div class="label">Slow threshold</div><div>{{.Report.QueryTime}}</div></div>
    <div class="metric"><div class="label">Findings</div><div>{{.FindingCount}}</div></div>
  </section>
  {{if .Report.Findings}}
    {{range .Report.Findings}}
      <section class="finding">
        <h2>{{.Operation.Namespace}} <span class="warning">{{runtimeMS .Operation}}</span></h2>
        <p><strong>Operation:</strong> {{.Operation.Operation}} · <strong>ID:</strong> {{.Operation.ID}}</p>
        <h3>Query</h3>
        <pre>{{prettyCommand .Operation.Command}}</pre>
        {{if .ExplainSkippedReason}}
          <p class="warning"><strong>Explain skipped:</strong> {{.ExplainSkippedReason}}</p>
        {{else if .ExplainError}}
          <p class="warning"><strong>Explain failed:</strong> {{.ExplainError}}</p>
        {{else}}
          <h3>Execution Stats</h3>
          <table>
            <tr><th>Stage</th><td>{{.Stats.Stage}}</td></tr>
            <tr><th>Documents examined</th><td>{{.Stats.TotalDocsExamined}}</td></tr>
            <tr><th>Keys examined</th><td>{{.Stats.TotalKeysExamined}}</td></tr>
            <tr><th>Documents returned</th><td>{{.Stats.NReturned}}</td></tr>
            <tr><th>Blocking sort</th><td>{{.Stats.HasBlockingSort}}</td></tr>
          </table>
          <h3>Reason and Fix</h3>
          <table>
            <tr><th>Reason</th><th>How to fix it</th></tr>
            {{range .Diagnostics}}
              <tr><td>{{.Reason}}</td><td>{{.Fix}}</td></tr>
            {{end}}
          </table>
        {{end}}
      </section>
    {{end}}
  {{else}}
    <section class="finding">
      <h2>No slow operations detected</h2>
      <p>No active operations exceeded the configured slow-query threshold during this scan.</p>
    </section>
  {{end}}
</main>
</body>
</html>
`))
