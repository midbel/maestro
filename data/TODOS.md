// TODOS syntax
// # section
// * code[(tag list...)]: short description
// multine description with optional leading space
// - property: value
// <<: marks an item as "done"
// >>: marks an item as "in progress"

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

# NICE TO HAVE

* maestro(feature): prefix output with the name of the command being executed
  set stdout/stderr of Command.Shell with os.Pipe/io.Pipe

* maestro(command,decoder,execute): marks dependencies as "optional"
  errors returned by these commands are ignored and execution can continue

* maestro(command,decoder,execute): conditionally executing dependendies and/or commands

* maestro(decoder,environment): predefined functions to "transform" value(s)

* maestro(macro): ellipsis operator (...) in repeat macro
  variable to be considered as a list of values
  operator syntax is TBD

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
