module github.com/midbel/maestro

go 1.18

require (
	github.com/midbel/distance v0.1.0
	github.com/midbel/slices v0.8.0
	github.com/midbel/textwrap v0.1.2
	github.com/midbel/tish v0.1.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
)

require (
	github.com/midbel/rw v0.3.0 // indirect
	github.com/midbel/shlex v0.2.2 // indirect
)

replace github.com/midbel/slices v0.8.0 => ../slices

replace github.com/midbel/shlex v0.2.2 => ../shlex
