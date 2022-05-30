package options

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"

	serverconfig "github.com/gocrane/crane-scheduler/pkg/server/config"
)

// ServerOptions used for web server
type ServerOptions struct {
	BindAddress string
	BindPort    int

	EnableProfiling bool
	EnableMetrics   bool
	Mode            string
	StoreType       string
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{}
}

func (o *ServerOptions) Complete() error {
	return nil
}

func (o *ServerOptions) ApplyTo(cfg *serverconfig.Config) error {
	cfg.BindAddress = o.BindAddress
	cfg.BindPort = o.BindPort

	cfg.Mode = o.Mode
	cfg.EnableMetrics = o.EnableMetrics
	cfg.EnableProfiling = o.EnableProfiling
	return nil
}

func (o *ServerOptions) Validate() []error {
	var errors []error
	if o.BindPort < 0 || o.BindPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--server-bind-port %v must be between 0 and 65535, inclusive. 0 for turning off insecure (HTTP) port",
				o.BindPort,
			),
		)
	}

	return errors
}

// AddFlags adds flags related to features for a specific server option to the
// specified FlagSet.
func (o *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}

	fs.StringVar(&o.BindAddress, "server-bind-address", "0.0.0.0", ""+
		"The IP address on which to serve the --server-bind-port "+
		"(set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")
	fs.IntVar(&o.BindPort, "server-bind-port", 8088,
		"The port on which to serve unsecured, unauthenticated access")

	fs.BoolVar(&o.EnableProfiling, "server-enable-profiling", o.EnableProfiling,
		"Enable profiling via web interface host:port/debug/pprof/")

	fs.BoolVar(&o.EnableMetrics, "server-enable-metrics", o.EnableMetrics,
		"Enables metrics on the server at /metrics")

	fs.StringVar(&o.Mode, "server-mode", gin.ReleaseMode,
		"Debug mode of the gin server, support release,debug,test")
}
