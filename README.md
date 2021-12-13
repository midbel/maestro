# maestro

maestro helps to organize all the tasks and/or commands that need to be performed regularly in a project whatever its nature. It could be the development of a program, administration of a single server or a set of virtual machines,...

### maestro file

#### comment

a hash symbol marks the rest of the line as a comment (except when inside of a string).

```
# a comment
ident = value # another comment
```

inside command scripts, comments are written in the same way as elsewhere in the document.

#### basic types

maestro supports only three different primitive types:

* `literal`: a literal is a sequence of characters that start with a letter and then contains only letters (lower/uppercase), digits and/or underscore.
* `string`: a string is a sequence of characters that starts with a single or a double quote and ends with the same opening quote. There are no differences in the way characters inside the string are process regarding of the delimiting quotes.
* `boolean`: they are just the commons values we are used to - `true` or `false` (always lowercase).

However, in some circumstances, maestro expects that values written as literal and/or string can be casted to integer values and/or duration values.

There is also a special case for boolean values. Indeed, depending of the context a boolean value will be considered as boolean but in some other case, the value of the boolean will be treated as a literal value.

Finally, maestro supports also multiline strings by using a syntax identical to the one of heredoc string of `bash`

some examples:
```
boolean = true
ident   = literal
single_quote = 'the quick brown fox jumps over the lazy dog'  
double_quote = "the quick brown fox jumps over the lazy dog"
heredoc = <<HELP
the quick brown fox
jumps over
the lazy dog!
HELP
```

maestro supports also a list of strings type (kind of array of string). To declare it, just provide a sequence of values separated by blank character (space or tab)

example:
```
list = first second third
```

#### variables

```
mode = dev # single value variable
package = shell shlex wrap # multi values variable
bindir = $bin # set value of variable bin to bindir

expansion = $(echo foo bar)
```

#### meta

meta are a special kind of variables that are used by maestro in order to generate the help of the input file, specify options for SSH execution, list of commands to be executed (default, all commands, before, after),...

* `.AUTHOR`: author of the maestro file
* `.EMAIL`: e-mail of the author of the maestro file
* `.VERSION`: current version of the maestro file
* `.USAGE`: short help message of the maestro file
* `.HELP`: longer description of the maestro file and description of its commands/usage
* `.DUPLICATE`: behaviour of maestro when it encounters a command with a name already registered. The possible values are:
  - error: throw an error if a command with the same name is already registered
  - replace: replace the previous definition of a command by the new one
  - append:  make the two commands as one
* `.TRACE`: enable/disabled tracing information
* `.WORKDIR`: set the working directory of maestro to the given path
* `.ALL`: list of commands that will be executed when calling `maestro all`
* `.DEFAULT`: name of the command that will be executed when calling `maestro` without argument or by calling `maestro default`
* `.BEFORE`: list of commands that will always be executed before the called command and its dependencies
* `.AFTER`: list of commands that will always be executed after the called command has finished whatever its exit status
* `.ERROR`: list of commands that will be executed after the called command has finished and its exit status is non zero (failure)
* SUCCESS: list of commands that will be executed after the called command has finished and its exit status is zero (success)
* `.SSH_USER`: username to use when executing command to remote server(s) via SSH
* `.SSH_PASSWORD`: password to use when executing command to remote server(s) via SSH
* `.SSH_PARALLEL`: number of instance of a command that will be executed simultaneously
* `.SSH_PUBKEY`: public key file to use when executing command to remote server(s) via SSH
* `.SSH_KNOWN_HOSTS`: known_hosts file to use to validate remote server(s) key

#### instructions

##### include

the `include` instruction allows to "include" the command of other files into the set of the current one.

the syntax to include file(s) is:
```
include "path/to/file.mf"[?]
# or if multiple files should be included:
include (
  "path/to/file1.mf"[?]
  ...
  "path/to/file1N.mf"[?]
)
```

the question mark modifier at the end of the filename specifies that the include is optional. In other words, if the given file can not be found, no error will be returned and the processing of the maestro file will continue.

Moreover, the files will be searched relative to the paths given with -I option of the maestro command. If the file can be found, then the file will be searched relatived to the current working directory or the directory set via the `.WORKDIR` meta.

There is an additional feature regarding included file that can be a little bit counter intuitive.

When maestro includes a file, it creates a new state from its local state before starting decoding the included files. All variables defined into the included files will be stored into this children state and as soon as maestro gets back to the original file, this sub state is discarded and references to variables of the included files are removed.

That means that variables defined in a file that should be included by maestro are  only visible to the commands defined into the included file and can not be resolved by variables and/or commands defined into the "parent" file.

##### export

the `export` instruction register variables as environment variables that will be given to each command that will be executed in command scripts

the syntax to `export` variable is:
```
export IDENT=VALUE0 ... VALUEN
# or
export (
  IDENT = VALUE
  ...
  IDENT = VALUE
)
```

##### alias

the `alias` instruction has the same role as defining an alias within a shell.

the syntax of `alias` declaration is:
```
alias ident = command
# or
alias (
  ident = command
  ...
  ident = command
)
```

##### delete

the `delete` instruction can be used to delete from the locals state of maestro variable previously defined

the syntax of `delete` declaration is:
```
delete ident0 ... identN
```

#### Command

Commands are at the heart of maestro. They are composed of four parts:

* the command name that serve as uniquely identify the command
* properties are used by maestro to generate help of a command but they are also used by maestro to control the behaviour of the commands in case of errors, long running tasks,...
* dependendies are a list of command's name that should be executed every time before a specific command is executed
* the command script that are the actual code that will be run by maestro

general syntax:

```
[%]command([property,...]): [dependency...] {
  [modifier...]script
}
```

##### command properties

* `short`: short description of a command
* `help`: longer description of a command.
* `tag`:  list of tags to help categorize a command in comparison with other
* `alias`: list of alternative name of a command
* `workdir`: set working directory for the command
* `retry`: number of attempts to run a command
* `timeout`: maximum time given to a command in order to fully complete
* `error`: behavior of maestro when the command encounters an error. The possible values are:
  - silent: ignore all error
  - error: return the first error encounters
* `user`: list of users allowed to run a command
* `group`: list of groups allowed to run a command
* `options`: list of list that describes the options accepted by a command
* `args`: list of names that describes the arguments required by a command
* `hosts`: list of remote servers where a command can be executed. The expected syntax is host:port

##### command options and arguments

maestro allows to define the options and/or arguments that a command can accept. In the properties section of a command, there is only needs to specify the `options` and/or the `args` properties.

The `options` property accept a list of list with the following property:

* `short`: short option
* `long`: long option
* `desc`: description of the option
* `flag`: wheter the option is a flag or is expecting a value
* `required`: wheter a value should be provided
* `default`: default value to use if the option is not set

For the `args` property, only a list of name is needed. The command when executed will expect that the number of arguments given matched the number of arguments given in the list. If the `args` property is not defined then any given arguments will be given to the command without checking its number.

example
```
action(
	short   = "sample action", # inline comment
	tag     = example,
	options = (
		short    = "a",
		long     = "all",
		flag     = true,
	), (
		short    = "b",
		long     = "bind",
    required = true,
	)
): {
	script...
}
```

##### command dependencies

for each command defined in the maestro file, it is possible to give a list of command dependencies - command that should be executed before the actual command get executed. Moreover, if one of the command dependencies failed, the command called will not be executed. A dependency is another command defined in the maestro file or one of the include file(s).

the syntax to specify a dependency is:
```
[!]depname[(arguments...)][&]
```

where

* `!`: specify that the dependency is optional and any errors returned by it will be ignored
* `depname`: is the name of the command
* `arguments`: a list of arguments (mix of options + their values and arguments) that should be given to the command
* [&]: wheter the command can be run into the background and its results does not impact the result of successfull command in the list. If the command runs in background returns an error, the rest of the dependency list and the actual command won't be executed

##### command help

even if there is already a `desc` property to command in order to specify the help of a command. It can be tedious to write a multiline string in the properties declaration of a command. Of course, we can use a variable and assign a heredoc string and then assign the variable to the `desc` property.

However, it exists a last way to do it. This ways is inspired by python docstring. To specify the help of a command, you can use comment at the very beginning of the command scripts to generate the command help.

example
```
action(properties...): dependencies...{
  # this is a sample action.
  # the first comments in the command script will be used as the description
  # of the command
  #
  # blank lines will be kept in the final formatting. However, multiple blank lines
  # will be merge as one blank line
  script...
}
```

##### command script

the script is at the heart of a command. it is as the name suggest the actual script that maestro should execute in order to accomplish the task.

a script for maestro is a succession of line. Each line can be composed of two things: the script modifier (which are describes below) and the actual commands to be executed.

maestro support also the use of predefined macros in script (see below for more information). Macros are the way for maestro to modify a script. Transformations of macro are applied to the command scripts before the command gets executed.

general syntax:
```
[modifier]command [option] <arguments> [\]
```

if a line is too long, a backslash follow by a new line character at the end of the line forces maestro to read the next line as part of the current line.

maestro executes each line of a script individually and applies if specified the modifiers to command to be executed. If a line returns an error, then maestro ends the execution of the script and exit with a non-zero exit code.

###### modifiers

a script modifier is a way to attach specific behaviour to the script when it is executed or after it has finished:

* `-`: ignore errors if script ends with a non-zero exit code
* `!`: reverse the exit code of a command. If the exit code is zero then a non zero value is returned
* `@`: print on stdout the command being executed
* `<`: copy the full script of a command defined elsewhere in the maestro file

multiple operators can be used simultaneously. except for the copy modifier that can only be used alone

examples:

```
!-echo foobar
@echo foobar
<copy
```

###### repeat macro

```
.repeat(foo bar) {
  echo <iter> <var>
}
```
will be transform by maestro into
```
echo 1 foo
echo 2 bar
```

###### sequence macro

the sequence macro can be used to transformed a multiline sequence of a command as a single command made of a list of command.

eg:
```
.sequence {
  echo foo
  echo bar
}
```
will be transformed into:
```
echo foo; echo bar;
```

#### example

```makefile
.VERSION = "0.1.0"
.DEFAULT = test
.ALL     = test
.AUTHOR  = midbel
.EMAIL   = "noreply@midbel.dev"
.HELP    = <<HELP
simple is a sample maestro file that can be used for any Go project.
It provides commands to automatize building, checking and testing
your Go project.
HELP

package = "github.com/midbel/maestro"

test(
	short = "run test in current directory",
	tag   = build test,
): {
  # run go test
	go clean -testcache
	@go test -cover $package
	@go test -cover "$package/shlex"
	@go test -cover "$package/shell"
}
```

### command execution

### maestro shell

in order to execute all the command and their scripts, maestro does not called an external shell such as bash or zsh... Indeed, maestro uses its own shell with its own rules, set of builtins and the rest...

This section will describes the supported features of the maestro shell. This shell is also available as a separated binary called tish (tiny shell).

#### general syntax

#### shell expansions

##### variables

##### parameters expansions

##### braces expansions

##### arithmetic expansions

##### command subsitutions

#### shell commands

##### simple commands

##### list of commands

##### pipelines

##### loop constructs

##### conditional constructs
