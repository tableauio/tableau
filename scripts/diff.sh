#!/bin/bash

# Check if the user provided at least two arguments
if [ $# -lt 2 ]; then
  echo "Usage: $0 <old_dir> <new_dir> [file_format]"
  exit 1
fi

old_dir="$1"
new_dir="$2"

# Check if the file format is specified
if [ $# -eq 3 ]; then
  file_format="$3"
  find_pattern="-name \"*.${file_format}\""
else
  find_pattern=""
fi

# Find all files with the specified format in the old directory
files=$(eval find "$old_dir" -type f $find_pattern)

# Initialize counters
total_files=0
diff_files=0

# Loop through the files and check for differences
for file in $files; do
  # Get the corresponding file in the new directory
  new_file="${file/$old_dir/$new_dir}"

  # Check if the new file exists
  if [ -e "$new_file" ]; then
    # Compare the files and print the differences
    diff_output=$(diff -u "$file" "$new_file")
    if [ -n "$diff_output" ]; then
      echo "Differences found in $file and $new_file:"
      echo "$diff_output"
      diff_files=$((diff_files + 1))
    fi
    total_files=$((total_files + 1))
  else
    echo "New file not found: $new_file"
  fi
done

# Print the final stats
echo "Total files compared: $total_files"
echo "Files with differences: $diff_files"
