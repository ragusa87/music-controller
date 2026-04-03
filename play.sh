#!/bin/bash
set -e

if [ "$1" = "" ]; then
 echo "Please add a term to play"
 echo "$0 <term>"
  echo "$0 random"
 exit 1
fi

if [ "$1" = "random" ]; then
  NUMBER=${2:-5}
  mpc clear

  while read -r i; do
    echo $i
    #mpc search artist "$i" | grep local | mpc add;
    mpc search artist "$i" | grep -v spotify | mpc add;
  done <<< "`mpc list artist | shuf | head -n $NUMBER | tac`" # | tr -s ' ' '+'`"
  mpc play
  exit 0
fi

name=`echo "$1"` # | tr -s ' ' '+'`
echo "Search $name"
mpc clear

# Remove spotify artists from selection (for mopidy's spotify plugin)
mpc search artist "$name" | grep -v spotify | mpc add
mpc play
