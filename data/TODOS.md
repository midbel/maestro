// TODOS syntax
// # section
// * [modifier]code[(tag list...)]: short description
// multine description with optional leading space
// - property: value
// <<: marks an item as "done"
// >>: marks an item as "in progress"
// !: marks an item as ignored
// ? marks an item as suspended

# TODOS

* maestro(feature): implements the ListenAndServe
  stdout/stderr of Command.Shell should be set to the http.ResponseWriter given in the http.Handler
  - date: 2021-11-30
  - version: 0.1.0
  - author: midbel

* maestro(feature): implements ssh remote execution
  - date: 2021-11-30
  - version: 0.1.0
  - author: midbel

* <<maestro(feature,decoder,syntax): append operator
  syntax: variable += values...
  - date: 2021-11-30
  - version: 0.1.x
  - author: midbel

* shell(expander): implements ExpandSlice.Expand
  - date: 2021-11-30
  - version: 0.1.x
  - author: midbel

# BUGS

* maestro(feature,command): cancel command execution
  once a signal is sent to maestro, all commands being executed and the following one
  should be discarded/cancelled properly
  better use of context.Context and cancel
  - date: 2021-12-01
  - version: 0.1.0
  - author: midbel

* >>maestro(feature,command): skip dependencies/before/after command when -h flag is set
  when -h/--help flag is set, only the help of the command being executed should
  be printed.
  dependencies and others commands should not be executed and maestro should exit
  directly after having printed the help message
  - date: 2021-12-01
  - version: 0.1.0
  - author: midbel

# NICE TO HAVE

* maestro(command,decoder,execute): marks dependencies as "optional"
  errors returned by these commands are ignored and execution can continue
  syntax: !dep-name

* maestro(command,decoder,execute): conditionally executing dependendies and/or commands

* maestro(decoder,environment): predefined functions to "transform" value(s)

* <<maestro(macro): expand operator in repeat macro
  variable to be considered as a list of values
  operator syntax: variable+ (variable name + plus sign)

* maestro(feature): remote maestro file hosted on a web server
  use -f to retrieve a maestro file located on a remote web server
  commands retrieved from the remote file have to be executed on the remote server

# ENHANCEMENTS/IMPROVEMENTS

* shell(expansion): filename expansion
  check for special character
  resolving current and parent directories

* shell(expansion): check the quoted status of each Expander

* shell(expansion): escaped character
  check if special character has been escaped before performing any expansion

* >>maestro(feature): prefix output with the name of the command being executed
  revise type and mechanism used to output the results of the commands
