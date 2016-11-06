// +build pflag

package gofig

import (
	"fmt"
	"io"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// config contains the configuration information
type config struct {
	v                         *viper.Viper
	flagSets                  map[string]*pflag.FlagSet
	disableEnvVarSubstitution bool
}

func newConfigObj() *config {
	return &config{
		v:                         viper.New(),
		flagSets:                  map[string]*pflag.FlagSet{},
		disableEnvVarSubstitution: DisableEnvVarSubstitution,
	}
}

// Config is the interface that enables retrieving configuration information.
// The variations of the Get function, the Set, IsSet, and Scope functions
// all take an interface{} as their first parameter. However, the param must be
// either a string or a fmt.Stringer, otherwise the function will panic.
type Config interface {

	// DisableEnvVarSubstitution is the same as the global flag,
	// DisableEnvVarSubstitution.
	DisableEnvVarSubstitution(disable bool)

	// Parent gets the configuration's parent (if set).
	Parent() Config

	// FlagSets gets the config's flag sets.
	FlagSets() map[string]*pflag.FlagSet

	// Scope returns a scoped view of the configuration. The specified scope
	// string will be used to prefix all property retrievals via the Get
	// and Set functions. Please note that the other functions will still
	// operate as they would for the non-scoped configuration instance. This
	// includes the AllSettings and AllKeys functions as well; they are *not*
	// scoped.
	Scope(scope interface{}) Config

	// GetScope returns the config's current scope (if any).
	GetScope() string

	// GetString returns the value associated with the key as a string
	GetString(k interface{}) string

	// GetBool returns the value associated with the key as a bool
	GetBool(k interface{}) bool

	// GetStringSlice returns the value associated with the key as a string
	// slice.
	GetStringSlice(k interface{}) []string

	// GetInt returns the value associated with the key as an int
	GetInt(k interface{}) int

	// Get returns the value associated with the key
	Get(k interface{}) interface{}

	// Set sets an override value
	Set(k interface{}, v interface{})

	// IsSet returns a flag indicating whether or not a key is set.
	IsSet(k interface{}) bool

	// Copy creates a copy of this Config instance
	Copy() (Config, error)

	// ToJSON exports this Config instance to a JSON string
	ToJSON() (string, error)

	// ToJSONCompact exports this Config instance to a compact JSON string
	ToJSONCompact() (string, error)

	// MarshalJSON implements the encoding/json.Marshaller interface. It allows
	// this type to provide its own marshalling routine.
	MarshalJSON() ([]byte, error)

	// ReadConfig reads a configuration stream into the current config instance
	ReadConfig(in io.Reader) error

	// ReadConfigFile reads a configuration files into the current config
	// instance
	ReadConfigFile(filePath string) error

	// EnvVars returns an array of the initialized configuration keys as
	// key=value strings where the key is configuration key's environment
	// variable key and the value is the current value for that key.
	EnvVars() []string

	// AllKeys gets a list of all the keys present in this configuration.
	AllKeys() []string

	// AllSettings gets a map of this configuration's settings.
	AllSettings() map[string]interface{}
}

func (c *config) FlagSets() map[string]*pflag.FlagSet {
	return c.flagSets
}

func (c *config) processRegKeys(r ConfigRegistration) {
	fsn := fmt.Sprintf("%s Flags", r.Name())
	fs, ok := c.flagSets[fsn]
	if !ok {
		fs = &pflag.FlagSet{}
		c.flagSets[fsn] = fs
	}

	for k := range r.Keys() {

		if fs.Lookup(k.FlagName()) != nil {
			continue
		}

		evn := k.EnvVarName()

		if LogRegKey {
			log.WithFields(log.Fields{
				"keyName":      k.KeyName(),
				"keyType":      k.KeyType(),
				"flagName":     k.FlagName(),
				"envVar":       evn,
				"defaultValue": k.DefaultValue(),
				"usage":        k.Description(),
			}).Debug("adding flag")
		}

		// bind the environment variable
		c.v.BindEnv(k.KeyName(), evn)

		if k.Short() == "" {
			switch k.KeyType() {
			case String, SecureString:
				fs.String(k.FlagName(), k.DefaultValue().(string), k.Description())
			case Int:
				fs.Int(k.FlagName(), k.DefaultValue().(int), k.Description())
			case Bool:
				fs.Bool(k.FlagName(), k.DefaultValue().(bool), k.Description())
			}
		} else {
			switch k.KeyType() {
			case String, SecureString:
				fs.StringP(k.FlagName(), k.Short(), k.DefaultValue().(string), k.Description())
			case Int:
				fs.IntP(k.FlagName(), k.Short(), k.DefaultValue().(int), k.Description())
			case Bool:
				fs.BoolP(k.FlagName(), k.Short(), k.DefaultValue().(bool), k.Description())
			}
		}

		c.v.BindPFlag(k.KeyName(), fs.Lookup(k.FlagName()))
	}
}
