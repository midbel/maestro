var = test
case $var in
foo)
  echo foo
  echo $var
bar)
  echo bar
  echo $var
  if $var; then
    echo ok
  fi
qux,mux,bux)
  echo qux
  echo $var
*)
  echo "oups"
  for i in $(seq 1 5); do
    echo "$var: $i"
  done
  find -type f -name "*sh"
esac
