module github.com/midbel/maestro

go 1.18

require (
	github.com/google/go-cmp v0.5.9
	github.com/midbel/distance v0.1.0
	github.com/midbel/slices v0.8.0
	github.com/midbel/textwrap v0.1.2
	github.com/midbel/try v1.2.0
)

replace github.com/midbel/slices v0.8.0 => ../slices

replace github.com/midbel/shlex v0.2.2 => ../shlex
