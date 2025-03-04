package telemetry

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.flipt.io/flipt/internal/config"
	"go.flipt.io/flipt/internal/info"
	"go.uber.org/zap/zaptest"

	"gopkg.in/segmentio/analytics-go.v3"
)

var _ analytics.Client = &mockAnalytics{}

type mockAnalytics struct {
	msg        analytics.Message
	enqueueErr error
	closed     bool
}

func (m *mockAnalytics) Enqueue(msg analytics.Message) error {
	m.msg = msg
	return m.enqueueErr
}

func (m *mockAnalytics) Close() error {
	m.closed = true
	return nil
}

type mockFile struct {
	io.Reader
	io.Writer
}

func (m *mockFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (m *mockFile) Truncate(_ int64) error {
	return nil
}

func TestNewReporter(t *testing.T) {
	var (
		cfg = config.Config{
			Meta: config.MetaConfig{
				TelemetryEnabled: true,
			},
		}

		logger        = zaptest.NewLogger(t)
		reporter, err = NewReporter(cfg, logger, "foo", info.Flipt{})
	)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
}

func TestShutdown(t *testing.T) {
	var (
		logger        = zaptest.NewLogger(t)
		mockAnalytics = &mockAnalytics{}

		reporter = &Reporter{
			cfg: config.Config{
				Meta: config.MetaConfig{
					TelemetryEnabled: true,
				},
			},
			logger:   logger,
			client:   mockAnalytics,
			shutdown: make(chan struct{}),
		}
	)

	err := reporter.Shutdown()
	assert.NoError(t, err)

	assert.True(t, mockAnalytics.closed)
}

func TestPing(t *testing.T) {
	test := []struct {
		name string
		cfg  config.Config
		want map[string]interface{}
	}{
		{
			name: "basic",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					Protocol: config.DatabaseSQLite,
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "file",
				},
			},
		},
		{
			name: "with db url",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					URL: "sqlite:///foo.db",
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "sqlite",
				},
			},
		},
		{
			name: "with unknown db url",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					URL: "foo:///foo.db",
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "unknown",
				},
			},
		},
		{
			name: "with cache not enabled",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					Protocol: config.DatabaseSQLite,
				},
				Cache: config.CacheConfig{
					Enabled: false,
					Backend: config.CacheRedis,
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "file",
				},
			},
		},
		{
			name: "with cache",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					Protocol: config.DatabaseSQLite,
				},
				Cache: config.CacheConfig{
					Enabled: true,
					Backend: config.CacheRedis,
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "file",
					"cache":    "redis",
				},
			},
		},
		{
			name: "with auth not enabled",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					Protocol: config.DatabaseSQLite,
				},
				Authentication: config.AuthenticationConfig{
					Required: false,
					Methods: config.AuthenticationMethods{
						Token: config.AuthenticationMethod[config.AuthenticationMethodTokenConfig]{
							Enabled: false,
						},
					},
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "file",
				},
			},
		},
		{
			name: "with auth",
			cfg: config.Config{
				Database: config.DatabaseConfig{
					Protocol: config.DatabaseSQLite,
				},
				Authentication: config.AuthenticationConfig{
					Required: false,
					Methods: config.AuthenticationMethods{
						Token: config.AuthenticationMethod[config.AuthenticationMethodTokenConfig]{
							Enabled: true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"version": "1.0.0",
				"storage": map[string]interface{}{
					"database": "file",
				},
				"authentication": map[string]interface{}{
					"methods": []interface{}{
						"token",
					},
				},
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var (
				logger        = zaptest.NewLogger(t)
				mockAnalytics = &mockAnalytics{}
			)

			cfg := tt.cfg
			cfg.Meta.TelemetryEnabled = true

			var (
				reporter = &Reporter{
					cfg:    cfg,
					logger: logger,
					client: mockAnalytics,
					info: info.Flipt{
						Version: "1.0.0",
					},
				}

				in       = bytes.NewBuffer(nil)
				out      = bytes.NewBuffer(nil)
				mockFile = &mockFile{
					Reader: in,
					Writer: out,
				}
			)

			err := reporter.ping(context.Background(), mockFile)
			assert.NoError(t, err)

			msg, ok := mockAnalytics.msg.(analytics.Track)
			require.True(t, ok)
			assert.Equal(t, "flipt.ping", msg.Event)
			assert.NotEmpty(t, msg.AnonymousId)
			assert.Equal(t, msg.AnonymousId, msg.Properties["uuid"])
			assert.Equal(t, "1.1", msg.Properties["version"])
			assert.Equal(t, tt.want, msg.Properties["flipt"])

			assert.NotEmpty(t, out.String())
		})
	}
}

func TestPing_Existing(t *testing.T) {
	var (
		logger        = zaptest.NewLogger(t)
		mockAnalytics = &mockAnalytics{}

		reporter = &Reporter{
			cfg: config.Config{
				Meta: config.MetaConfig{
					TelemetryEnabled: true,
				},
			},
			logger: logger,
			client: mockAnalytics,
			info: info.Flipt{
				Version: "1.0.0",
			},
		}

		b, _     = ioutil.ReadFile("./testdata/telemetry_v1.json")
		in       = bytes.NewReader(b)
		out      = bytes.NewBuffer(nil)
		mockFile = &mockFile{
			Reader: in,
			Writer: out,
		}
	)

	err := reporter.ping(context.Background(), mockFile)
	assert.NoError(t, err)

	msg, ok := mockAnalytics.msg.(analytics.Track)
	require.True(t, ok)
	assert.Equal(t, "flipt.ping", msg.Event)
	assert.Equal(t, "1545d8a8-7a66-4d8d-a158-0a1c576c68a6", msg.AnonymousId)
	assert.Equal(t, "1545d8a8-7a66-4d8d-a158-0a1c576c68a6", msg.Properties["uuid"])
	assert.Equal(t, "1.1", msg.Properties["version"])
	assert.Equal(t, "1.0.0", msg.Properties["flipt"].(map[string]interface{})["version"])

	assert.NotEmpty(t, out.String())
}

func TestPing_Disabled(t *testing.T) {
	var (
		logger        = zaptest.NewLogger(t)
		mockAnalytics = &mockAnalytics{}

		reporter = &Reporter{
			cfg: config.Config{
				Meta: config.MetaConfig{
					TelemetryEnabled: false,
				},
			},
			logger: logger,
			client: mockAnalytics,
			info: info.Flipt{
				Version: "1.0.0",
			},
		}
	)

	err := reporter.ping(context.Background(), &mockFile{})
	assert.NoError(t, err)

	assert.Nil(t, mockAnalytics.msg)
}

func TestPing_SpecifyStateDir(t *testing.T) {
	var (
		logger = zaptest.NewLogger(t)
		tmpDir = os.TempDir()

		mockAnalytics = &mockAnalytics{}

		reporter = &Reporter{
			cfg: config.Config{
				Meta: config.MetaConfig{
					TelemetryEnabled: true,
					StateDirectory:   tmpDir,
				},
			},
			logger: logger,
			client: mockAnalytics,
			info: info.Flipt{
				Version: "1.0.0",
			},
		}
	)

	path := filepath.Join(tmpDir, filename)
	defer os.Remove(path)

	err := reporter.report(context.Background())
	assert.NoError(t, err)

	msg, ok := mockAnalytics.msg.(analytics.Track)
	require.True(t, ok)
	assert.Equal(t, "flipt.ping", msg.Event)
	assert.NotEmpty(t, msg.AnonymousId)
	assert.Equal(t, msg.AnonymousId, msg.Properties["uuid"])
	assert.Equal(t, "1.1", msg.Properties["version"])
	assert.Equal(t, "1.0.0", msg.Properties["flipt"].(map[string]interface{})["version"])

	b, _ := ioutil.ReadFile(path)
	assert.NotEmpty(t, b)
}
