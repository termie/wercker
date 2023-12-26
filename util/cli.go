//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// This is an interface and a helper to make it easier to construct our options
// objects for testing without literally parsing the flags we define in
// the wercker/cmd package. Mostly it is a re-implementation of the codegangsta
// cli.Context interface that we actually use.
// I'm sure there is a better way to do this but damn if I'm not tired of typing
// out boilerplate for settings types and defaults.

package util

import (
	"time"

	cli "gopkg.in/urfave/cli.v1"
)

// Settings mathces the cli.Context interface so we can make a cheap
// re-implementation for testing purposes.
type Settings interface {
	Int(string, ...interface{}) (int, bool)
	Duration(string, ...interface{}) (time.Duration, bool)
	Float64(string, ...interface{}) (float64, bool)
	Bool(string, ...interface{}) (bool, bool)
	BoolT(string, ...interface{}) (bool, bool)
	String(string, ...interface{}) (string, bool)
	StringSlice(string, ...interface{}) ([]string, bool)
	IntSlice(string, ...interface{}) ([]int, bool)

	GlobalInt(string, ...interface{}) (int, bool)
	GlobalDuration(string, ...interface{}) (time.Duration, bool)
	// NOTE(termie): for some reason not in cli.Context
	// GlobalFloat64(string, ...interface{}) (float64, bool)
	GlobalBool(string, ...interface{}) (bool, bool)
	// NOTE(termie): for some reason not in cli.Context
	// GlobalBoolT(string, ...interface{}) (bool, bool)
	GlobalString(string, ...interface{}) (string, bool)
	GlobalStringSlice(string, ...interface{}) ([]string, bool)
	GlobalIntSlice(string, ...interface{}) ([]int, bool)
}

// cheapValue saves us some boilerplate
type cheapValue struct {
	value interface{}
	found bool
}

func (v *cheapValue) Int() (rv int, ok bool) {
	if r, asserted := v.value.(int); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) Duration() (rv time.Duration, ok bool) {
	if r, asserted := v.value.(time.Duration); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) Float64() (rv float64, ok bool) {
	if r, asserted := v.value.(float64); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) Bool() (rv bool, ok bool) {
	if r, asserted := v.value.(bool); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) String() (rv string, ok bool) {
	if r, asserted := v.value.(string); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) StringSlice() (rv []string, ok bool) {
	if r, asserted := v.value.([]string); asserted {
		return r, v.found
	}
	return rv, false
}

func (v *cheapValue) IntSlice() (rv []int, ok bool) {
	if r, asserted := v.value.([]int); asserted {
		return r, v.found
	}
	return rv, false
}

func lookup(name string, data map[string]interface{}, def ...interface{}) *cheapValue {
	var v interface{}
	if len(def) == 1 {
		v = def[0]
	}
	if d, found := data[name]; found {
		return &cheapValue{d, true}
	}
	return &cheapValue{v, false}
}

// CheapSettings based on a map, returns val, ok
type CheapSettings struct {
	data map[string]interface{}
}

func NewCheapSettings(data map[string]interface{}) *CheapSettings {
	return &CheapSettings{data}
}

func (s *CheapSettings) Int(name string, def ...interface{}) (rv int, ok bool) {
	return lookup(name, s.data, def...).Int()
}

func (s *CheapSettings) Duration(name string, def ...interface{}) (rv time.Duration, ok bool) {
	return lookup(name, s.data, def...).Duration()
}

func (s *CheapSettings) Float64(name string, def ...interface{}) (rv float64, ok bool) {
	return lookup(name, s.data, def...).Float64()
}

func (s *CheapSettings) Bool(name string, def ...interface{}) (rv bool, ok bool) {
	return lookup(name, s.data, def...).Bool()
}

// BoolT is true by default
func (s *CheapSettings) BoolT(name string, def ...interface{}) (rv bool, ok bool) {
	// this would be sort of stupid to do considering you are using a BoolT... but... hey
	if len(def) == 1 {
		return lookup(name, s.data, def...).Bool()
	}
	return lookup(name, s.data, true).Bool()
}

func (s *CheapSettings) String(name string, def ...interface{}) (rv string, ok bool) {
	return lookup(name, s.data, def...).String()
}

func (s *CheapSettings) StringSlice(name string, def ...interface{}) (rv []string, ok bool) {
	return lookup(name, s.data, def...).StringSlice()
}

func (s *CheapSettings) IntSlice(name string, def ...interface{}) (rv []int, ok bool) {
	return lookup(name, s.data, def...).IntSlice()
}

// All the Global versions to do the same thing as the non-global
func (s *CheapSettings) GlobalInt(name string, def ...interface{}) (rv int, ok bool) {
	return s.Int(name, def...)
}

func (s *CheapSettings) GlobalDuration(name string, def ...interface{}) (rv time.Duration, ok bool) {
	return s.Duration(name, def...)
}

func (s *CheapSettings) GlobalBool(name string, def ...interface{}) (rv bool, ok bool) {
	return s.Bool(name, def...)
}

func (s *CheapSettings) GlobalString(name string, def ...interface{}) (rv string, ok bool) {
	return s.String(name, def...)
}

func (s *CheapSettings) GlobalStringSlice(name string, def ...interface{}) (rv []string, ok bool) {
	return s.StringSlice(name, def...)
}

func (s *CheapSettings) GlobalIntSlice(name string, def ...interface{}) (rv []int, ok bool) {
	return s.IntSlice(name, def...)
}

// CLISettings is a wrapper on a cli.Context with a special "target" set
// in place of "Args().First()"
type CLISettings struct {
	c             *cli.Context
	CheapSettings *CheapSettings
}

func NewCLISettings(ctx *cli.Context) *CLISettings {
	return &CLISettings{
		ctx,
		&CheapSettings{map[string]interface{}{"target": ctx.Args().First()}},
	}
}

func (s *CLISettings) Int(name string, def ...interface{}) (rv int, ok bool) {
	if v, ok := s.CheapSettings.Int(name, def...); ok {
		return v, ok
	}
	return s.c.Int(name), s.c.IsSet(name)
}

func (s *CLISettings) Duration(name string, def ...interface{}) (rv time.Duration, ok bool) {
	if v, ok := s.CheapSettings.Duration(name, def...); ok {
		return v, ok
	}
	return s.c.Duration(name), s.c.IsSet(name)
}

func (s *CLISettings) Float64(name string, def ...interface{}) (rv float64, ok bool) {
	if v, ok := s.CheapSettings.Float64(name, def...); ok {
		return v, ok
	}
	return s.c.Float64(name), s.c.IsSet(name)
}

func (s *CLISettings) Bool(name string, def ...interface{}) (rv bool, ok bool) {
	if v, ok := s.CheapSettings.Bool(name, def...); ok {
		return v, ok
	}
	return s.c.Bool(name), s.c.IsSet(name)
}

func (s *CLISettings) BoolT(name string, def ...interface{}) (rv bool, ok bool) {
	if v, ok := s.CheapSettings.BoolT(name, def...); ok {
		return v, ok
	}
	return s.c.BoolT(name), s.c.IsSet(name)
}

func (s *CLISettings) String(name string, def ...interface{}) (rv string, ok bool) {
	if v, ok := s.CheapSettings.String(name, def...); ok {
		return v, ok
	}
	return s.c.String(name), s.c.IsSet(name)
}

func (s *CLISettings) StringSlice(name string, def ...interface{}) (rv []string, ok bool) {
	if v, ok := s.CheapSettings.StringSlice(name, def...); ok {
		return v, ok
	}
	return s.c.StringSlice(name), s.c.IsSet(name)
}

func (s *CLISettings) IntSlice(name string, def ...interface{}) (rv []int, ok bool) {
	if v, ok := s.CheapSettings.IntSlice(name, def...); ok {
		return v, ok
	}
	return s.c.IntSlice(name), s.c.IsSet(name)
}

func (s *CLISettings) GlobalInt(name string, def ...interface{}) (rv int, ok bool) {
	if v, ok := s.CheapSettings.Int(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalInt(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalDuration(name string, def ...interface{}) (rv time.Duration, ok bool) {
	if v, ok := s.CheapSettings.Duration(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalDuration(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalBool(name string, def ...interface{}) (rv bool, ok bool) {
	if v, ok := s.CheapSettings.Bool(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalBool(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalString(name string, def ...interface{}) (rv string, ok bool) {
	if v, ok := s.CheapSettings.String(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalString(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalStringSlice(name string, def ...interface{}) (rv []string, ok bool) {
	if v, ok := s.CheapSettings.StringSlice(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalStringSlice(name), s.c.GlobalIsSet(name)
}

func (s *CLISettings) GlobalIntSlice(name string, def ...interface{}) (rv []int, ok bool) {
	if v, ok := s.CheapSettings.IntSlice(name, def...); ok {
		return v, ok
	}
	return s.c.GlobalIntSlice(name), s.c.GlobalIsSet(name)
}
