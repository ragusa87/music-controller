#!/bin/bash
set -e
# Make sure the "module-combine-sink" is loaded
# then set it as default and redirect all sinks to it

# List sinks
echo "SINKS:"
pactl list sinks | awk '/Name|device.description|Source|Sink/ { print $0 }'

# List inputs
echo ""
echo "Inputs"
pactl list sink-inputs | awk '/Sink:|Client:|media\.name|application.name/ {print $0};'
echo ""

# Make sure the module is loaded
cmd=$(pactl list | grep combined)
if [ -z "$cmd" ]; then
  pactl load-module module-combine-sink sink_name=combined
else
   echo "combine module already loaded"
fi

# Find the dual source's id
DUAL_ID=$(pactl list sinks short | grep combine | head -1 |awk '{print $1}')
echo pactl set-default-sink $DUAL_ID
pactl set-default-sink $DUAL_ID


# Fins the first sink native
ID=$(pactl list sink-inputs short | grep native | head -1 | awk '{print $1}')

# Find all sink with another source than dual_id that are not combined
IDS=$(pactl list sink-inputs short | grep -v combine | grep -E -v "^[0-9]+\s+[$DUAL_ID]" | awk '{ print $1 }' | xargs)
done=0
for i in $IDS
do
 done=1
 echo "pactl move-sink-input $i $DUAL_ID"
 pactl move-sink-input $i $DUAL_ID
done

if [ $done -eq 0 ]; then
 echo "All inputs are already ok"
fi

