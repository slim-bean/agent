package metrics

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/agent/component"
	component_config "github.com/grafana/agent/component/common/config"
	"github.com/grafana/agent/component/common/loki"
	"github.com/grafana/agent/component/discovery"
	"github.com/grafana/agent/component/prometheus"
)

func init() {
	component.Register(component.Registration{
		Name:    "loki.source.metrics",
		Args:    Arguments{},
		Exports: Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the prometheus.scrape
// component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `river:"forward_to,attr"`

	IncludeMetrics []string `river:"include_metrics,attr,optional"`
	PromoteLabels  []string `river:"promote_labels,attr,optional"`

	HTTPClientConfig component_config.HTTPClientConfig `river:",squash"`
}

type Exports struct {
	Receiver storage.Appendable `river:"receiver,attr"`
}

// Component implements the loki.source.syslog component.
type Component struct {
	opts component.Options

	reloadTargets chan struct{}

	mut       sync.RWMutex
	args      Arguments
	receivers []loki.LogsReceiver
	targets   []discovery.Target

	buf *bytes.Buffer

	handler loki.LogsReceiver
}

// New creates a new loki.source.syslog component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:      o,
		handler:   loki.NewLogsReceiver(),
		receivers: args.ForwardTo,
		args:      args,
	}

	receiver := prometheus.NewInterceptor(
		nil,
		prometheus.WithAppendHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			if len(c.args.IncludeMetrics) > 0 {
				allowed := false
				name := l.Get(model.MetricNameLabel)
				for _, a := range c.args.IncludeMetrics {
					if name == a {
						allowed = true
						break
					}
				}
				if !allowed {
					return globalRef, nil
				}
			}

			// This seemed necessary because without it we got some bad corruption of labels/values in Loki
			lbls := l.Copy()

			// Remove the __name__ label from the output because we store it in a label
			// having it in the log line is redundant and uses a lot of space
			nameLabelIdx := 0
			for i, label := range lbls {
				if label.Name == model.MetricNameLabel {
					nameLabelIdx = i
					break
				}
			}
			nameLabel := lbls[nameLabelIdx]
			lbls = append(lbls[:nameLabelIdx], lbls[nameLabelIdx+1:]...)

			lbls = append(lbls, labels.Label{
				Name:  "val",
				Value: fmt.Sprintf("%v", v),
			})

			indexedLabels := model.LabelSet{
				"metric":       model.LabelValue(nameLabel.Value),
				"__encoding__": "metric",
			}

			buf := bytes.NewBuffer(nil)
			for i, lbl := range lbls {
				for _, a := range c.args.PromoteLabels {
					if lbl.Name == a {
						indexedLabels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
					}
				}

				fmt.Fprint(buf, lbl.Name)
				fmt.Fprint(buf, "=")
				writeStringValue(buf, lbl.Value)
				if i < len(lbls)-1 {
					fmt.Fprint(buf, " ")
				} else {
					fmt.Fprint(buf, "\n")
				}
			}

			c.handler.Chan() <- loki.Entry{
				Labels: indexedLabels,
				Entry: logproto.Entry{
					Timestamp: timestamp.Time(t),
					Line:      buf.String(),
				},
			}

			return globalRef, nil
		}),
	)

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: receiver})

	return c, nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	return nil
}

func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.receivers {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

func needsQuotedValueRune(r rune) bool {
	return r <= ' ' || r == '=' || r == '"' || r == utf8.RuneError
}

var hex = "0123456789abcdef"

func writeStringValue(buf *bytes.Buffer, value string) {
	if value == "null" {
		buf.WriteString(`"null"`)
	} else if strings.IndexFunc(value, needsQuotedValueRune) != -1 {
		writeQuotedString(buf, value)
	} else {
		buf.WriteString(value)
	}
}

func writeQuotedString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if 0x20 <= b && b != '\\' && b != '"' {
				i++
				continue
			}
			if start < i {
				buf.WriteString(s[start:i])
			}
			switch b {
			case '\\', '"':
				buf.WriteByte('\\')
				buf.WriteByte(b)
			case '\n':
				buf.WriteByte('\\')
				buf.WriteByte('n')
			case '\r':
				buf.WriteByte('\\')
				buf.WriteByte('r')
			case '\t':
				buf.WriteByte('\\')
				buf.WriteByte('t')
			default:
				// This encodes bytes < 0x20 except for \n, \r, and \t.
				buf.WriteString(`\u00`)
				buf.WriteByte(hex[b>>4])
				buf.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			if start < i {
				buf.WriteString(s[start:i])
			}
			buf.WriteString(`\ufffd`)
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf.WriteString(s[start:])
	}
	buf.WriteByte('"')
}
