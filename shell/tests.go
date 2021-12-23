package shell

var testops = map[string]rune{
	"-eq": Eq,
	"-ne": Ne,
	"-lt": Lt,
	"-le": Le,
	"-gt": Gt,
	"-ge": Ge,
	"-nt": NewerThan,
	"-ot": OlderThan,
	"-ef": SameFile,
	"-e":  FileExists,
	"-r":  FileRead,
	"-h":  FileLink,
	"-d":  FileDir,
	"-w":  FileWrite,
	"-s":  FileSize,
	"-f":  FileRegular,
	"-x":  FileExec,
	"-z":  StrNotEmpty,
	"-n":  StrEmpty,
}
