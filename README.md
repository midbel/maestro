# maestro

maestro helps to organize all the tasks and/or commands that need to be performed regularly in a project whatever its nature. It could be the development of a program, administration of a single server or a set of virtual machines,...

### changelog

#### v0.4.0

* conditional dependency(ies)/commands (pre-conditions such as OS, files available...)
* hazardous command property will cause a prompt of the password of the current user
* schedule property `preserve` to always use the same shell context when starting new execution of the same command
* lint sub-command
* define object variable
* others...

#### v0.3.0

below the list of additions/modifications/deletions that will be introduce in the `v0.3.0`:

* new sub-command: schedule
* `schedule` command property related to the future `schedule` sub-command
* reload of maestro file for the `serve` sub-command
* variable interpolation in string
* namespaced command: command in file(s) can be namespaced to not combined them with others having same name from same or included files
* support for the `case` instruction in script command
* handling of command that must expand on multiple lines in script command (eg: for loop, if...) without introducing any macro
* better handling of shell and subshell management during command execution
* `Combined` Command type to be revised
* fill shell builtins
* others...

#### v0.2.0 (2022-01-22)

below, the list of additions/modifications/deletions introduces in the `v0.2.0`:

* internal of command execution rewritten
* command(s) can be directly called inside the script of other command(s)
* command options and arguments validation
* mandatory dependency(ies)
* new sub-commands: serve, graph
* support for test expression in command script
* support for `break` and `continue` keyword in command script
* minor modifications in the execution of commands via ssh
* command suggestion(s) when given command is not known (typo,...)
* improved error message when syntax error is found when decoding input file

### maestro file

this section describes the syntax and features offered by a maestro file to write and organize your commands.

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

the repeat macro has three special variables that will be replaced by their actual once the macro get executed:

* `<var>`: the current identifier/variable being process
* `<iter>`: the current iteration of the repeat macro with 1 based index
* `<iter0>`: the current iteration of the repeat macro with 0 based index

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

maestro use the term `shell` even if its shell does not implement a compliant shell.

This section will describes the supported features of the maestro shell. This shell is also available as a separated binary called tish (tiny shell).

To get a complete overview of the shell syntax, you can read the manual of one of the well known shell

#### general syntax

Most of the syntax of the maestro shell (aka tish) is inspired by bash. However, in comparison of bash, all the rules have been overly simplify and the maestro shell does not follow systematically and formerly the same rules of bash and other well known shells. So if you're an experienced bash/shell programmer, you can/will be regularly surprised by the behaviour of the maestro shell.

the maestro shell split command lines on blank characters. Blank characters are: a space, a tab and a newline. When multiple blank characters follow each other, they are considered as one.

##### quoting

As traditional shells, the maestro shell has special behaviours when it encounters quotes and adapts more or less the same behaviour: disabling meaning of special character.

enclosing string between single quote character keeps the literal value of each character between the quotes.

enclosing string between double quote only keeps the special meaning of the dollar sign $ (variables, parameters expansion, command substitution and arithmetic expansion) and preserves the literal value of all others characters.

#### shell expansions

the maestro shell supports 7 kind of expansions described below. It performs expansion in the order they appear in the command - from left to right. It is very different than the way traditional shells perform expansion.

##### literals/words

when literal word are encountered, they are kept as is by the maestro shell except if special character are present in the literal (`*.[`). If such characters appear in the string then maestro shell will performed filename expansion at the very end if the literal is not quoted.

##### variables

as any other shells and programming language, the maestro shell supports the definition and usage of variables. Variables are identifier where you can store value to be used later in your script.

A variable starts with a dollar character and its name. Its name can only be composed of underscore, digits and ascii letters (lowercase and uppercase).

example:
```bash
echo $VAR
```

to assign a value to a variable, uses the following syntax:

```bash
VAR = foobar
```

##### parameters expansions

parameters expansions can take multiple form:

* length
* replace prefix/suffix/substring/all
* slicing
* trim prefix/suffix
* padding
* lowercase and uppercase

example:
```bash
$ ${#VAR} # length
$ ${VAR:offset:length} # slicing offset+length
$ ${VAR/from/to} # replace the first instance of from by to
$ ${VAR//from/to} # replace all instances of from by to
$ ${VAR/%from/to} # replace suffix instance of from by to
$ ${VAR/#from/to} # replace prefix instance of from by to
$ ${VAR%suffix} # trim suffix
$ ${VAR%%suffix} # trim longest suffix
$ ${VAR#prefix} # trim prefix
$ ${VAR##prefix} # trim longest prefix
$ ${VAR,} # set the first character of VAR to lowercase
$ ${VAR,,} # set all characters of VAR to lowercase
$ ${VAR^} # set the first character of VAR to uppercase
$ ${VAR^^} # set all characters of VAR to uppercase
$ ${VAR:<length:char} # pad left VAR with length char
$ ${VAR:>length:char} # pad right VAR with length char
```

##### braces expansions

braces expansions is a sequence introduces by the `{}` operator and can take two forms:

* as a list
* as a range (with an optional step)

example:
```bash
$ {foo,bar}
$ foo bar
$ {1..10}
$ 1 2 3 4 5 6 7 8 9 10
$ {1..10..3}
$ 1 4 7 10
```

##### arithmetic expansions

the maestro shell allows arithmetric expression to be evaluated and generates one word resulting of the computation of the expression

it supports most of the arithmetic expression supported by any well known shell and/or programming language.

syntax:
```bash
$(( expression ))
```
operators:

- `++`, `--`: increment, decrement operators
- `+`, `-`: addition, subtraction (also unary minus)
- `*`, `/`, `%`: multiplication, division, modulo
- `**`: power
- `<<`, `>>`: left and right shift
- `&`, `|`, `~`: binary and, binary or, xor (also binary not)
- `&&`, `||`: relational and, or
- `==`, `!=`: equality operator
- `<`, `<=`, `>`, `>=`: comparison operator
- `?:` : conditional (ternary) operator
- `()`: expression group

##### command substitutions

command substitution is the execution of a command where everything written to stdout by the command is then splitted on blank characters.

example:
```bash
echo $(echo foo bar)
```

##### filename expansions

filename expansion is like any other filename expansion. After having expanded all others form of expansions, the maestro shell looks in each expanded word for special character (`*.[`). If one of these characters is found, the word is then considered as a pattern. Then, for each pattern, the maestro shell will try to find all files which their filenames match the given pattern.

#### shell commands

##### simple commands

a simple command is the simplest form of command understood by the maestro shell. It is composed of a list of words separated by blank characters and resulting of the expansion of each of the individual tokens of the line.

##### list of commands

a list of commands is simply the concatenation of simple command on the same line. each command is separated by a semicolon character

##### pipelines

##### loop constructs

tish supports three differents looping constructs:

* the `for` loop
* the `while` loop
* the `until` loop

in addition, the maestro shell foreseen an optional `else` clause for each loop when no iteration has been performed.

the `for` loop iterates throught the list of expanded words given and then executes each commands in the body of loop. If the expansion of words returned an empty list then the `else` clause of the loop is executed.

when the second form is used, the `for` loop will directly iterates of the results of expanded words from the output of the command. Again, if no words are expanded, then the `else` clause is executed.

```bash
for ident in words; do
  commands;
else
  alternative-commands;
done
# or
for command; do
  commands;
else
  alternative-commands;
done
```

the `while` loop will executes each commands in the body of the loop while the executed command returns a zero exit code. If the command returns directly a non zero exit code, then the `else` clause will be executed.

```bash
while command; do
  commands
else
  alternative-commands;
done
```

the `until` loop is the exact opposite of the `while` loop. The body of the loop will be executed while the command returns a non-zero exit code. If the command returns directly a zero exit code, then the `else` clause is executed

```bash
until command; do
  commands
else
  alternative-commands;
done
```

##### conditional constructs

the maestro shell currently only supports the `if`. Support for the `case` constructs is forseen for a later release.

```bash
if command; then
  commands
elif command; then
  commands
else
  commands
fi
```

##### redirections

like traditional shell, the maestro shell supports redirections. However it does not support all kind of redirections supported by well known shells.

```bash
$ command < file # redirect file to stdin of command
$ command > file # redirect stdout of command to file
$ command >> file # redirect stdout of command and append to file
$ command 2> file # redirect stderr of command to file
$ command 2>> file # redirect stderr of command and append to file
$ command &> file # redirect stdout and stderr of command to file
$ command &>> file # redirect stdout and stderr of command and append to file
```

As an additional note, the order of how redirections are written is important. Indeed, if you try to redirect twice stdout/stderr/stdin to different files, only the latest declaration will be taken into account and the previous will be discarded.

Last note, if using any kind of expansions (described above) the resulting expansion should only be expanded to one and only one word otherwise an error is returned.


##### builtins
