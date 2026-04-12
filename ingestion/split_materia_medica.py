#!/usr/bin/env python3
"""Split Materia_Medica.md into separate files per remedy section."""

import re
import os

INPUT_FILE = os.path.join(os.path.dirname(__file__), "articles", "Materia_Medica.md")
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "articles")

# Pattern to match START markers, e.g.:
#   **[START OF INFORMATION REGARDING ANACARDIUM]{.underline}**
START_PATTERN = re.compile(
    r'^\*\*\[?START OF INFORMATION REGARDING (.+?)(?:\]\{\.underline\})?\*\*\s*$'
)
# Pattern to match END markers (some may lack {.underline})
END_PATTERN = re.compile(
    r'^\*\*\[?END OF INFORMATION REGARDING (.+?)(?:\]\{\.underline\})?\*\*\s*$'
)


def main():
    with open(INPUT_FILE, "r", encoding="utf-8") as f:
        lines = f.readlines()

    current_name = None
    current_lines = []
    sections_written = 0

    for line in lines:
        start_match = START_PATTERN.match(line.strip())
        end_match = END_PATTERN.match(line.strip())

        if start_match:
            current_name = start_match.group(1).strip()
            current_lines = [line]
        elif end_match:
            if current_lines:
                current_lines.append(line)
                # Convert name to filename: spaces -> underscores
                filename = current_name.replace(" ", "_") + ".md"
                filepath = os.path.join(OUTPUT_DIR, filename)
                with open(filepath, "w", encoding="utf-8") as out:
                    out.writelines(current_lines)
                sections_written += 1
                print(f"  Written: {filename} ({len(current_lines)} lines)")
                current_name = None
                current_lines = []
        elif current_name is not None:
            current_lines.append(line)

    print(f"\nDone. {sections_written} sections extracted to {OUTPUT_DIR}")


if __name__ == "__main__":
    main()
