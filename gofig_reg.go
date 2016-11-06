package gofig

import (
	"bytes"
	"strings"
	"unicode"

	log "github.com/Sirupsen/logrus"
	"github.com/akutz/goof"
)

// ConfigRegistration is an interface that describes a configuration
// registration object.
type ConfigRegistration interface {

	// Name returns the name of the config registration.
	Name() string

	// YAML returns the registration's default yaml configuration.
	YAML() string

	// SetYAML sets the registration's default yaml configuration.
	SetYAML(y string)

	// Key adds a key to the registration.
	//
	// The first vararg argument is the yaml name of the key, using a '.' as
	// the nested separator. If the second two arguments are omitted they will
	// be generated from the first argument. The second argument is the explicit
	// name of the flag bound to this key. The third argument is the explicit
	// name of the environment variable bound to thie key.
	Key(
		keyType int,
		short string,
		defVal interface{},
		description string,
		keys ...interface{})

	// Keys returns a channel on which a listener can receive the config
	// registration's keys.
	Keys() <-chan ConfigRegistrationKey
}

// ConfigRegistrationKey is an interfact that describes a cofniguration
// registration key object.
type ConfigRegistrationKey interface {
	KeyType() int
	DefaultValue() interface{}
	Short() string
	Description() string
	KeyName() string
	FlagName() string
	EnvVarName() string
}

type configReg struct {
	name string
	yaml string
	keys []ConfigRegistrationKey
}

type configRegKey struct {
	keyType    int
	defVal     interface{}
	short      string
	desc       string
	keyName    string
	flagName   string
	envVarName string
}

const (
	// String is a key with a string value
	String = iota // 0

	// Int is a key with an integer value
	Int // 1

	// Bool is a key with a boolean value
	Bool // 2

	// SecureString is a key with a string value that is not included when the
	// configuration is marshaled to JSON.
	SecureString // 3
)

// NewRegistration creates a new registration with the given name.
func NewRegistration(name string) ConfigRegistration {
	return newRegistration(name)
}

func newRegistration(name string) *configReg {
	return &configReg{name: name, keys: []ConfigRegistrationKey{}}
}

func (r *configReg) Name() string {
	return r.name
}

func (r *configReg) Keys() <-chan ConfigRegistrationKey {
	c := make(chan ConfigRegistrationKey)
	go func() {
		for _, k := range r.keys {
			c <- k
		}
		close(c)
	}()
	return c
}

func (r *configReg) YAML() string     { return r.yaml }
func (r *configReg) SetYAML(y string) { r.yaml = y }

func (r *configReg) Key(
	keyType int,
	short string,
	defVal interface{},
	description string,
	keys ...interface{}) {

	lk := len(keys)
	if lk == 0 {
		panic(goof.New("keys is empty"))
	}

	rk := &configRegKey{
		keyType: keyType,
		short:   short,
		desc:    description,
		defVal:  defVal,
		keyName: toString(keys[0]),
	}

	if keyType == SecureString {
		secureKey(rk)
	}

	if lk < 2 {
		kp := strings.Split(rk.keyName, ".")
		for x, s := range kp {
			if x == 0 {
				var buff []byte
				b := bytes.NewBuffer(buff)
				for y, r := range s {
					if y == 0 {
						b.WriteRune(unicode.ToLower(r))
					} else {
						b.WriteRune(r)
					}
				}
				kp[x] = b.String()
			} else {
				kp[x] = strings.Title(s)
			}
		}
		rk.flagName = strings.Join(kp, "")
	} else {
		rk.flagName = toString(keys[1])
	}

	if lk < 3 {
		kp := strings.Split(rk.keyName, ".")
		for x, s := range kp {
			kp[x] = strings.ToUpper(s)
		}
		rk.envVarName = strings.Join(kp, "_")
	} else {
		rk.envVarName = toString(keys[2])
	}

	r.keys = append(r.keys, rk)
}

func (k *configRegKey) KeyType() int              { return k.keyType }
func (k *configRegKey) DefaultValue() interface{} { return k.defVal }
func (k *configRegKey) Short() string             { return k.short }
func (k *configRegKey) Description() string       { return k.desc }
func (k *configRegKey) KeyName() string           { return k.keyName }
func (k *configRegKey) FlagName() string          { return k.flagName }
func (k *configRegKey) EnvVarName() string        { return k.envVarName }

func secureKey(k *configRegKey) {
	secureKeysRWL.Lock()
	defer secureKeysRWL.Unlock()
	kn := strings.ToLower(k.keyName)
	if LogSecureKey {
		log.WithField("keyName", kn).Debug("securing key")
	}
	secureKeys[kn] = k
}
