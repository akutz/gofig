/*
Package gofig simplifies external, runtime configuration of go programs.
*/
package gofig

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"reflect"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	errors "github.com/akutz/goof"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	homeDirPath   string
	etcDirPath    string
	usrDirPath    string
	envVarRx      *regexp.Regexp
	registrations []*Registration
	prefix        string
)

func init() {
	envVarRx = regexp.MustCompile(`^\s*([^#=]+?)=(.+)$`)
	loadEtcEnvironment()
}

// Config contains the configuration information
type Config struct {
	FlagSets map[string]*flag.FlagSet `json:"-"`
	v        *viper.Viper
}

// SetGlobalConfigPath sets the path of the directory from which the global
// configuration file is read.
func SetGlobalConfigPath(path string) {
	etcDirPath = path
}

// SetUserConfigPath sets the path of the directory from which the user
// configuration file is read.
func SetUserConfigPath(path string) {
	usrDirPath = path
}

// Register registers a new configuration with the config package.
func Register(r *Registration) {
	registrations = append(registrations, r)
}

// New initializes a new instance of a Config struct
func New() *Config {
	return NewConfig(true, true, "config", "yml")
}

// NewConfig initialies a new instance of a Config object with the specified
// options.
func NewConfig(
	loadGlobalConfig, loadUserConfig bool,
	configName, configType string) *Config {

	log.Debug("initializing configuration")

	c := &Config{
		v:        viper.New(),
		FlagSets: map[string]*flag.FlagSet{},
	}
	c.v.SetTypeByDefaultValue(false)
	c.v.SetConfigName(configName)
	c.v.SetConfigType(configType)

	c.processRegistrations()

	cfgFile := fmt.Sprintf("%s.%s", configName, configType)
	etcConfigFile := fmt.Sprintf("%s/%s", etcDirPath, cfgFile)
	usrConfigFile := fmt.Sprintf("%s/%s", usrDirPath, cfgFile)

	if loadGlobalConfig && fileExists(etcConfigFile) {
		log.WithField("path", etcConfigFile).Debug("loading global config file")
		if err := c.ReadConfigFile(etcConfigFile); err != nil {
			log.WithError(err).WithField("path", etcConfigFile).Debug(
				"error reading global config file")
		}
	}

	if loadUserConfig && fileExists(usrConfigFile) {
		log.WithField("path", usrConfigFile).Debug("loading user config file")
		if err := c.ReadConfigFile(usrConfigFile); err != nil {
			log.WithError(err).WithField("path", usrConfigFile).Debug(
				"error reading user config file")
		}
	}

	return c
}

func (c *Config) processRegistrations() {
	for _, r := range registrations {

		fs := &flag.FlagSet{}

		for _, k := range r.keys {

			evn := k.envVarName

			log.WithFields(log.Fields{
				"keyName":      k.keyName,
				"keyType":      k.keyType,
				"flagName":     k.flagName,
				"envVar":       evn,
				"defaultValue": k.defVal,
				"usage":        k.desc,
			}).Debug("adding flag")

			// bind the environment variable
			c.v.BindEnv(k.keyName, evn)

			if k.short == "" {
				switch k.keyType {
				case String:
					fs.String(k.flagName, k.defVal.(string), k.desc)
				case Int:
					fs.Int(k.flagName, k.defVal.(int), k.desc)
				case Bool:
					fs.Bool(k.flagName, k.defVal.(bool), k.desc)
				}
			} else {
				switch k.keyType {
				case String:
					fs.StringP(k.flagName, k.short, k.defVal.(string), k.desc)
				case Int:
					fs.IntP(k.flagName, k.short, k.defVal.(int), k.desc)
				case Bool:
					fs.BoolP(k.flagName, k.short, k.defVal.(bool), k.desc)
				}
			}

			c.v.BindPFlag(k.keyName, fs.Lookup(k.flagName))
		}

		c.FlagSets[r.name+" Flags"] = fs

		// read the config
		if r.yaml != "" {
			c.ReadConfig(bytes.NewReader([]byte(r.yaml)))
		}
	}
}

// Copy creates a copy of this Config instance
func (c *Config) Copy() (*Config, error) {
	newC := New()
	m := map[string]interface{}{}
	c.v.Unmarshal(&m)
	for k, v := range m {
		newC.v.Set(k, v)
	}
	return newC, nil
}

// FromJSON initializes a new Config instance from a JSON string
func FromJSON(from string) (*Config, error) {
	c := New()
	m := map[string]interface{}{}
	if err := json.Unmarshal([]byte(from), &m); err != nil {
		return nil, err
	}
	for k, v := range m {
		c.v.Set(k, v)
	}
	return c, nil
}

// ToJSON exports this Config instance to a JSON string
func (c *Config) ToJSON() (string, error) {
	buf, _ := json.MarshalIndent(c, "", "  ")
	return string(buf), nil
}

// ToJSONCompact exports this Config instance to a compact JSON string
func (c *Config) ToJSONCompact() (string, error) {
	buf, _ := json.Marshal(c)
	return string(buf), nil
}

// MarshalJSON implements the encoding/json.Marshaller interface. It allows
// this type to provide its own marshalling routine.
func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.allSettings())
}

// ReadConfig reads a configuration stream into the current config instance
func (c *Config) ReadConfig(in io.Reader) error {
	if in == nil {
		return errors.New("config reader is nil")
	}
	return c.v.MergeConfig(in)
}

// ReadConfigFile reads a configuration files into the current config instance
func (c *Config) ReadConfigFile(filePath string) error {
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return c.ReadConfig(bytes.NewBuffer(buf))
}

// EnvVars returns an array of the initialized configuration keys as key=value
// strings where the key is configuration key's environment variable key and
// the value is the current value for that key.
func (c *Config) EnvVars() []string {
	keyVals := c.allSettings()
	envVars := make(map[string]string)
	c.flattenEnvVars("", keyVals, envVars)
	var evArr []string
	for k, v := range envVars {
		evArr = append(evArr, fmt.Sprintf("%s=%v", k, v))
	}
	return evArr
}

// flattenEnvVars returns a map of configuration keys coming from a config
// which may have been nested.
func (c *Config) flattenEnvVars(
	prefix string, keys map[string]interface{}, envVars map[string]string) {

	for k, v := range keys {

		var kk string
		if prefix == "" {
			kk = k
		} else {
			kk = fmt.Sprintf("%s.%s", prefix, k)
		}
		ek := strings.ToUpper(strings.Replace(kk, ".", "_", -1))

		log.WithFields(log.Fields{
			"key":   kk,
			"value": v,
		}).Debug("flattening env vars")

		switch vt := v.(type) {
		case string:
			envVars[ek] = vt
		case []interface{}:
			var vArr []string
			for _, iv := range vt {
				vArr = append(vArr, iv.(string))
			}
			envVars[ek] = strings.Join(vArr, " ")
		case map[string]interface{}:
			c.flattenEnvVars(kk, vt, envVars)
		case bool:
			envVars[ek] = fmt.Sprintf("%v", vt)
		case int, int32, int64:
			envVars[ek] = fmt.Sprintf("%v", vt)
		}
	}
	return
}

// AllKeys gets a list of all the keys present in this configuration.
func (c *Config) AllKeys() []string {
	ak := []string{}
	as := c.allSettings()

	for k, v := range as {
		switch tv := v.(type) {
		case nil:
			continue
		case map[string]interface{}:
			flattenArrayKeys(k, tv, &ak)
		default:
			ak = append(ak, k)
		}
	}

	return ak
}

// AllSettings gets a map of this configuration's settings.
func (c *Config) AllSettings() map[string]interface{} {
	return c.allSettings()
}

func (c *Config) allSettings() map[string]interface{} {
	as := map[string]interface{}{}
	ms := map[string]map[string]interface{}{}

	for k, v := range c.v.AllSettings() {
		switch tv := v.(type) {
		case nil:
			continue
		case map[string]interface{}:
			ms[k] = tv
		default:
			as[k] = tv
		}
	}

	for msk, msv := range ms {
		flat := map[string]interface{}{}
		flattenMapKeys(msk, msv, flat)
		for fk, fv := range flat {
			if asv, ok := as[fk]; ok && reflect.DeepEqual(asv, fv) {
				log.WithFields(log.Fields{
					"key":     fk,
					"valAll":  asv,
					"valFlat": fv,
				}).Debug("deleting duplicate flat val")
				delete(as, fk)
			}
		}
		as[msk] = msv
	}

	return as
}

func flattenArrayKeys(
	prefix string, m map[string]interface{}, flat *[]string) {
	for k, v := range m {
		kk := fmt.Sprintf("%s.%s", prefix, k)
		switch vt := v.(type) {
		case map[string]interface{}:
			flattenArrayKeys(kk, vt, flat)
		default:
			*flat = append(*flat, kk)
		}
	}
}

func flattenMapKeys(
	prefix string, m map[string]interface{}, flat map[string]interface{}) {
	for k, v := range m {
		kk := fmt.Sprintf("%s.%s", prefix, k)
		switch vt := v.(type) {
		case map[string]interface{}:
			flattenMapKeys(kk, vt, flat)
		default:
			flat[strings.ToLower(kk)] = v
		}
	}
}

// GetString returns the value associated with the key as a string
func (c *Config) GetString(k string) string {
	return c.v.GetString(k)
}

// GetBool returns the value associated with the key as a bool
func (c *Config) GetBool(k string) bool {
	return c.v.GetBool(k)
}

// GetStringSlice returns the value associated with the key as a string slice
func (c *Config) GetStringSlice(k string) []string {
	return c.v.GetStringSlice(k)
}

// GetInt returns the value associated with the key as an int
func (c *Config) GetInt(k string) int {
	return c.v.GetInt(k)
}

// Get returns the value associated with the key
func (c *Config) Get(k string) interface{} {
	return c.v.Get(k)
}

// Set sets an override value
func (c *Config) Set(k string, v interface{}) {
	c.v.Set(k, v)
}

func loadEtcEnvironment() {
	lr := lineReader("/etc/environment")
	if lr == nil {
		return
	}
	for l := range lr {
		m := envVarRx.FindStringSubmatch(l)
		if m == nil || len(m) < 3 || os.Getenv(m[1]) != "" {
			continue
		}
		os.Setenv(m[1], m[2])
	}
}

// fileExists returns a flag indicating whether a provided file path exists.
func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		return true
	}
	return false
}

// lineReader returns a channel that reads the contents of a file line-by-line.
func lineReader(filePath string) <-chan string {
	if !fileExists(filePath) {
		return nil
	}

	c := make(chan string)
	go func() {
		f, _ := os.Open(filePath)
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			c <- s.Text()
		}
		close(c)
	}()
	return c
}

// homeDir returns the home directory of the user that owns the current process.
func homeDir() string {
	if homeDirPath != "" {
		return homeDirPath
	}
	if user, err := user.Current(); err == nil {
		homeDirPath = user.HomeDir
	}
	return homeDirPath
}
