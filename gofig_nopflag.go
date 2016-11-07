// +build !pflag

package gofig

import (
	"github.com/akutz/gofig/types"
	"github.com/spf13/viper"
)

// config contains the configuration information
type config struct {
	v                         *viper.Viper
	disableEnvVarSubstitution bool
}

func newConfigObj() *config {
	return &config{
		v: viper.New(),
		disableEnvVarSubstitution: DisableEnvVarSubstitution,
	}
}

func (c *config) processRegKeys(r types.ConfigRegistration) {
	for k := range r.Keys() {
		c.v.BindEnv(k.KeyName(), k.EnvVarName())
	}
}
