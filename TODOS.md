TODOS

* prefix output with the name of the command being executed: set stdout/stderr of Command.Shell with os.Pipe/io.Pipe
* implements the ListenAndServe
* implements ssh remote execution

BUGS

* filename expansion should only be performed once all the words in a list has been expanded otherwise parts will be lost

NICE TO HAVE

* marks dependencies as "optional": errors returned by these commands are ignored and execution can continue
* conditionally executing dependendies and/or commands
* predefined functions to "transform" literal/variables/... value(s)
* ellipsis operator (...) to work with the repeat macro: variable to be considered as a list of values
* use -f to retrieve a maestro file located on a remote web server. commands executed will be sent to the remote server
