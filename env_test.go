package maestro_test

import (
  "testing"

  "github.com/midbel/maestro"
)

func TestEnv(t *testing.T) {
  p := maestro.EmptyEnv()
  p.Define("foo", []string{"foo"})
  p.Define("bar", []string{"bar"})
  
  e := maestro.EnclosedEnv(p)
  e.Define("foobar", []string{"foo", "bar"})
  values, _ := e.Resolve("foobar")
  if values[0] != "foo" || values[1] != "bar" {
    t.Fatalf("values mismatched! got %v", values)
  }

  c := e.Copy()
  others, _ := c.Resolve("foobar")
  if others[0] != "foo" || others[1] != "bar" {
    t.Fatalf("values mismatched! got %v", others)
  }
  c.Define("test", []string{"test"})

  values, _ = e.Resolve("test")
  if len(values) != 0 {
    t.Fatalf("empty values expected! got %v", values)
  }
}
