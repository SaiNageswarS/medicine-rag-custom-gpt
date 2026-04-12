#!/usr/bin/env python3
"""Add markdown headings to remedy files split from Materia_Medica.md.
Skips Acontium.md and alumina.md."""

import re
import os
import glob

ARTICLES_DIR = os.path.join(os.path.dirname(__file__), "articles")

# Files to skip
SKIP_FILES = {"Acontium.md", "alumina.md", "Materia_Medica.md"}

# Patterns for lines that are "stand-alone header" lines
# These are lines like: **[SOMETHING]{.underline}** or ***[SOMETHING]{.underline}***
# that sit alone on a line and represent section/sub-section headers.
UNDERLINE_HEADER = re.compile(
    r'^(\*{2,3})\[(.+?)\]\{\.underline\}\1\s*$'
)

# Bold-only headers like **MIND.** or **CHARACTERISTICS.** or **GENERAL.**
# These typically appear in Morrison/Radar sections as sub-sections.
# Must be alone on a line, all-caps or title-case, short.
BOLD_HEADER = re.compile(
    r'^\*\*([A-Z][A-Z\s\-/&\'.]+?\.?)\*\*\s*$'
)

# START/END markers
START_MARKER = re.compile(r'^\*\*\[?START OF INFORMATION REGARDING (.+?)(?:\]\{\.underline\})?\*\*\s*$')
END_MARKER = re.compile(r'^\*\*\[?END OF INFORMATION REGARDING (.+?)(?:\]\{\.underline\})?\*\*\s*$')

# Major source sections (## level)
MAJOR_SECTIONS = [
    "NOTES BY GV", "G.V. NOTES", "G.V NOTES", "KEYNOTES BY G.V.",
    "KEYNOTES BY G.V", "KEYNOTES OF GV",
    "RADAR KEYNOTES",
    "ACUTE G.V. NOTES", "ACUTE G.V NOTES", "ACUTE NOTES BY GV",
    "ACUTE NOTES BY GV.", "ACUTE KEYNOTES BY G.V.",
    "Morrison Materia Medica", "Morrison Seminar",
    "MORRISON SEMINAR",
]

# Sub-sections under GV notes (### level)
SUB_SECTIONS = [
    "MENTAL", "PHYSICAL", "MODALITIES", "MODALITIES:",
]

# Sections that indicate a named essay/essence (## level)
ESSENCE_KEYWORDS = ["ESSENCE", "CHILDREN", "SEMINAR", "VASSILIS"]


def unescape_md(text):
    """Remove markdown escapes like \\# \\* \\-- from display text."""
    text = text.replace('\\#', '#').replace('\\*', '*').replace('\\--', '--')
    text = text.replace('\\-', '-').replace("\\'", "'")
    # Strip inner italic markers: *text* -> text
    text = re.sub(r'^\*(.+)\*$', r'\1', text.strip())
    text = re.sub(r'\*([^*]+)\*', r'\1', text)
    return text.strip()


def classify_header(text):
    """Determine heading level for an underlined header text.
    Returns (level, clean_title) or None if not a header."""
    
    # Strip remedy name prefix like "ARSENICUM ALBUM -- " 
    clean = unescape_md(text.strip())
    
    # Check if it's a START/END marker
    if "START OF INFORMATION" in clean or "END OF INFORMATION" in clean:
        return None
    
    # Remove leading # or * markers from the text
    display = re.sub(r'^[#*]\s*', '', clean).strip()
    
    # Remove remedy name prefix: "REMEDY -- # Title" or "REMEDY -- * Title"
    prefix_match = re.match(r'^[A-Z][A-Z\s\.]+\s*--\s*[#*]?\s*(.+)$', display)
    if prefix_match:
        display = prefix_match.group(1).strip()
    
    # Also handle "REMEDY -- Title" without # or *
    prefix_match2 = re.match(r'^[A-Z][A-Z\s\.]+\s*--\s*(.+)$', display)
    if prefix_match2:
        display = prefix_match2.group(1).strip()
    
    # Check major sections
    for ms in MAJOR_SECTIONS:
        if ms.lower() in display.lower():
            return (2, display)
    
    # Check sub-sections
    stripped_display = display.rstrip(':')
    if stripped_display in SUB_SECTIONS or stripped_display.rstrip(':') in [s.rstrip(':') for s in SUB_SECTIONS]:
        return (3, display.rstrip(':'))
    
    # Check essence/children/seminar sections
    for kw in ESSENCE_KEYWORDS:
        if kw.lower() in display.lower():
            return (2, display)
    
    # Lettered sub-sections like "A. ARS. ON THE MENTAL - EMOTIONAL LEVEL"
    if re.match(r'^[A-Z]\.\s', display):
        return (3, display)
    
    # Numbered sections like "1. ACUTE COMPLEMENTARY REMEDIES"
    if re.match(r'^\d+\.\s', display):
        return (3, display)
    
    # Sections that look like stage descriptions
    if re.match(r'^THE (FIRST|SECOND|THIRD|FOURTH|FIFTH) STAGE', display, re.IGNORECASE):
        return (3, display)
    
    # If it's all caps and reasonably short, likely a subsection
    if display.isupper() and len(display) < 80:
        return (3, display)
    
    # Title-case short headers
    if len(display.split()) <= 6 and not any(c.islower() for c in display.replace(' ', '')):
        return (3, display)
    
    return None


def classify_bold_header(text):
    """Check if a bold-only header line should become a ### heading.
    These appear in Morrison/Radar as body-part sections."""
    clean = text.strip().rstrip('.')
    # Common Morrison/Radar sub-section names
    known = {
        "MIND", "GENERAL", "GENERALITIES", "GENERALS",
        "HEAD", "EYE", "EYES", "EAR", "EARS", "NOSE",
        "FACE", "MOUTH", "TEETH", "THROAT",
        "STOMACH", "GASTRO-INTESTINAL", "GASTROINTESTINAL",
        "ABDOMEN", "RECTUM", "STOOL",
        "URINARY", "URINARY AND SEXUAL ORGANS",
        "MALE GENITALIA", "FEMALE GENITALIA", "GENITALIA",
        "CHEST", "RESPIRATION", "COUGH",
        "BACK", "EXTREMITIES", "SKIN", "SLEEP",
        "FEVER", "FOOD AND DRINKS",
        "CHARACTERISTICS", "CLINICAL", "RELATIONSHIP",
        "LARYNX/TRACHEA", "LARYNX", "NECK",
    }
    if clean in known:
        return (3, clean)
    return None


def get_remedy_name(filename):
    """Get display name from filename."""
    name = os.path.splitext(filename)[0]
    return name.replace("_", " ")


def process_file(filepath):
    filename = os.path.basename(filepath)
    if filename in SKIP_FILES:
        return False
    
    remedy_name = get_remedy_name(filename)
    
    with open(filepath, "r", encoding="utf-8") as f:
        lines = f.readlines()
    
    new_lines = []
    title_added = False
    
    for line in lines:
        stripped = line.strip()
        
        # Skip START/END markers
        if START_MARKER.match(stripped) or END_MARKER.match(stripped):
            continue
        
        # Add title before first real content
        if not title_added and stripped:
            new_lines.append(f"# {remedy_name}\n\n")
            title_added = True
        
        # Check underlined header pattern
        m = UNDERLINE_HEADER.match(stripped)
        if m:
            header_text = m.group(2)
            result = classify_header(header_text)
            if result:
                level, display = result
                new_lines.append(f"{'#' * level} {display}\n\n")
                continue
        
        # Check bold-only header pattern (for Morrison/Radar sub-sections)
        bm = BOLD_HEADER.match(stripped)
        if bm:
            header_text = bm.group(1).strip()
            result = classify_bold_header(header_text)
            if result:
                level, display = result
                new_lines.append(f"{'#' * level} {display}\n\n")
                continue
        
        new_lines.append(line)
    
    # Write back
    with open(filepath, "w", encoding="utf-8") as f:
        f.writelines(new_lines)
    
    return True


def main():
    files = sorted(glob.glob(os.path.join(ARTICLES_DIR, "*.md")))
    processed = 0
    for filepath in files:
        if process_file(filepath):
            processed += 1
            print(f"  Processed: {os.path.basename(filepath)}")
    print(f"\nDone. {processed} files processed.")


if __name__ == "__main__":
    main()
