// TODOS syntax
// # section
// * [modifier]code[(tag list...)]: short description
// multine description with optional leading space
// - property: value
// <: marks an item as "done"
// >: marks an item as "in progress"
// !: marks an item as ignored
// ?: marks an item as suspended
// list of related TODOS???

# TODOS

* maestro(feature): implements the ListenAndServe
  stdout/stderr of Command.Shell should be set to the http.ResponseWriter given in the http.Handler
  - date: 2021-11-30
  - version: 0.2.0
  - author: midbel

* <maestro(feature): implements ssh remote execution
  - date: 2021-11-30
  - version: 0.1.0
  - author: midbel

* <maestro(feature,decoder,syntax): append operator
  syntax: variable += values...
  - date: 2021-11-30
  - version: 0.1.0
  - author: midbel

* <shell(expander): implements ExpandSlice.Expand
  - date: 2021-11-30
  - version: 0.1.0
  - author: midbel

* <maestro(feature,command,decoder): decode dependency's arguments
  - date: 2021-12-02
  - version: 0.1.0
  - author: midbel

* <shell(feature): implements for loop and if statement
  - date: 2021-12-02
  - author: midbel

* command(feature): extend the command options by providing a way to validate the given value
  add a type and/or a validate property to the list of properties supported by the options properties.
  moreover, provides a list of "predefined" function to validate the function (eg: isFile, isDir,...)
  - date: 2021-12-12
  - version: 0.2.0
  - author: midbel

* maestro(decode, feature): add supports for variable expansion inside literal and literal string
  - date: 2021-12-12
  - version: 0.2.0
  - author: midbel

* maestro(decode, feature): improve error message when syntax error is found while decoding input file
  - date: 2021-12-12
  - version: 0.2.0
  - author: midbel

* <maestro(decode, feature): fix comma at end of line when using multine list of properties
  - date: 2021-12-12
  - version: 0.1.1
  - author: midbel

* maestro(feature): let user called a command as if it was an shell user defined command
  Single type should implements the shell.Command interface + commands should be registered into
  the shell of the command being executed
  - date: 2021-12-13
  - version: 0.2.0
  - author: midbel

* shell(feature): add support for the case statement
  - date: 2021-12-13
  - version: 0.2.0
  - author: midbel

* <shell(feature): add support for the test ([[ expr ]]) statement
  - date: 2021-12-13
  - version: 0.2.0
  - author: midbel

* <shell(feature): add support for break and continue keyword in loop
  - date: 2021-12-13
  - version: 0.2.0
  - author: midbel

* maestro(command, decode): add a flag to command to indicate that it can run multiple times
  a command once executed will have a flag toggle to avoid that it is re-executed a second time
  in, eg, its dependencies tree. However in certain circumstances, a command should be allow to run multiple times (in dependencies and/or http context).
  - date: 2022-01-07
  - version: 0.2.0
  - author: midbel

* maestro(feature): introduce variable that can contain objects
  such kind of variable can be used to eg reused common schedule object (see related improvements below). this variable could also allow the edition of their properties and/or their extension when assign to command properties and/or other variables
  - date: 2022-01-07
  - version: 0.2.0
  - author: midbel

# BUGS

* <maestro(feature,command): cancel command execution
  once a signal is sent to maestro, all commands being executed and the following one
  should be discarded/cancelled properly
  better use of context.Context and cancel
  - date: 2021-12-01
  - version: 0.1.0
  - author: midbel

* <maestro(feature,command): skip dependencies/before/after command when -h flag is set
  when -h/--help flag is set, only the help of the command being executed should
  be printed.
  dependencies and others commands should not be executed and maestro should exit
  directly after having printed the help message
  - date: 2021-12-01
  - version: 0.1.0
  - author: midbel

* <shell(execute): nil pointer dereference
  panic occur when calling StdCmd.Exit when trying a command that does not exist

# NICE TO HAVE

* <maestro(command,decoder,execute): marks dependencies as "optional"
  errors returned by these commands are ignored and execution can continue
  syntax: !dep-name

* maestro(command,decoder,execute): conditionally executing dependendies and/or commands
  - author: midbel

* maestro(decoder,environment): predefined functions to "transform" value(s)
  - author: midbel

* <maestro(macro): expand operator in repeat macro
  variable to be considered as a list of values
  operator syntax: variable+ (variable name + plus sign)

* maestro(feature): remote maestro file hosted on a web server
  use -f to retrieve a maestro file located on a remote web server
  commands retrieved from the remote file have to be executed on the remote server
  - date: 2021-12-08
  - author: midbel

* maestro(command): schedule command execution (like cron job)
  specify the name of a command to be execute and when it should be executed. use http request to reconfigure if needed the scheduled time
  - date: 2022-01-07
  - author: midbel

# ENHANCEMENTS/IMPROVEMENTS

* shell(expansion): filename expansion
  check for special character
  resolving current and parent directories

* <shell(expansion): check the quoted status of each Expander

* shell(expansion): escaped character
  check if special character has been escaped before performing any expansion

* <maestro(feature): prefix output with the name of the command being executed
  revise type and mechanism used to output the results of the commands

* <command(feature): improve formatting of help command
