#!/bin/bash
# Copy the 4 most recent Voice Memos to samples/
# Run in Terminal - you may need to grant Terminal "Full Disk Access" in
# System Settings > Privacy & Security > Full Disk Access

SOURCE="$HOME/Library/Group Containers/group.com.apple.VoiceMemos.shared/Recordings"
DEST="/Users/jpfortier/dev/gym/samples"

if [[ ! -d "$SOURCE" ]]; then
  echo "Voice Memos Recordings folder not found at: $SOURCE"
  exit 1
fi

# Get 4 most recent m4a files (by modification time)
count=0
for f in $(ls -t "$SOURCE"/*.m4a 2>/dev/null); do
  [[ $count -ge 4 ]] && break
  name=$(basename "$f")
  cp -v "$f" "$DEST/voice-memo-$((count+1))-${name}"
  ((count++))
done

if [[ $count -eq 0 ]]; then
  echo "No .m4a files found. Check that recordings exist and Terminal has Full Disk Access."
  exit 1
fi

echo "Copied $count recording(s) to $DEST"
