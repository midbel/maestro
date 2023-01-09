module github.com/midbel/maestro

go 1.18

require (
	github.com/google/go-cmp v0.5.9
	github.com/midbel/distance v0.1.0
	github.com/midbel/shlex v0.2.2
	github.com/midbel/slices v0.8.0
	github.com/midbel/textwrap v0.1.2
	github.com/midbel/try v1.2.0
	golang.org/x/crypto v0.5.0
	golang.org/x/sync v0.1.0
)

require golang.org/x/sys v0.4.0 // indirect

replace github.com/midbel/slices v0.8.0 => ../slices

replace github.com/midbel/shlex v0.2.2 => ../shlex
