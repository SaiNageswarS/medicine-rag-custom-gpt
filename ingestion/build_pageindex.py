#!/usr/bin/env python3
"""Build PageIndex tree structures for all markdown articles and store in MongoDB.

Usage:
    # 1. Install dependencies:
    git clone --depth 1 https://github.com/VectifyAI/PageIndex.git /tmp/PageIndex
    pip3 install --upgrade python-dotenv pymongo
    pip3 install --upgrade -r /tmp/PageIndex/requirements.txt
    export PYTHONPATH=/tmp/PageIndex

    # 2. Set environment variables in a .env file at the project root:
    #   OPENAI_API_KEY=your_key_here
    #   MONGO_URI=mongodb+srv://user:pass@cluster.mongodb.net

    # 3. Build index on all articles and ingest into MongoDB:
    python3 ingestion/build_pageindex.py --with-summaries --with-text

    # Other examples:
    python3 ingestion/build_pageindex.py --model gpt-4.1 --with-summaries --with-text
    python3 ingestion/build_pageindex.py --single ACONITUM.md           # process one file only
    python3 ingestion/build_pageindex.py --json-only                    # skip MongoDB, save JSON locally
    python3 ingestion/build_pageindex.py --json-only --single ACONITUM.md  # test locally on one file
"""

import argparse
import asyncio
import glob
import json
import os
import sys

from dotenv import load_dotenv

# Load .env from the project root (one level up from ingestion/)
load_dotenv(os.path.join(os.path.dirname(__file__), "..", ".env"))

from pageindex.page_index_md import md_to_tree

ARTICLES_DIR = os.path.join(os.path.dirname(__file__), "..", "articles")
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "..", "results")

MONGO_DB = "devinderhealthcare"
MONGO_COLLECTION = "pageindex_docs"


async def build_index_for_file(
    md_path: str,
    model: str,
    with_summaries: bool,
    with_text: bool,
    summary_token_threshold: int,
    thinning: bool,
    thinning_threshold: int,
) -> dict:
    """Build a PageIndex tree for a single markdown file."""
    tree = await md_to_tree(
        md_path=md_path,
        if_thinning=thinning,
        min_token_threshold=thinning_threshold,
        if_add_node_summary="yes" if with_summaries else "no",
        summary_token_threshold=summary_token_threshold,
        model=model,
        if_add_doc_description="yes" if with_summaries else "no",
        if_add_node_text="yes" if with_text else "no",
        if_add_node_id="yes",
    )
    return tree


async def main():
    parser = argparse.ArgumentParser(
        description="Build PageIndex tree structures for medicine articles"
    )
    parser.add_argument(
        "--model",
        type=str,
        default="gpt-4.1",
        help="LLM model to use (default: gpt-4.1)",
    )
    parser.add_argument(
        "--with-summaries",
        action="store_true",
        help="Generate LLM summaries for each node (costs API calls)",
    )
    parser.add_argument(
        "--with-text",
        action="store_true",
        help="Include full text content in each tree node",
    )
    parser.add_argument(
        "--summary-token-threshold",
        type=int,
        default=200,
        help="Token threshold below which text is used as-is instead of summarizing (default: 200)",
    )
    parser.add_argument(
        "--thinning",
        action="store_true",
        help="Apply tree thinning to merge small nodes",
    )
    parser.add_argument(
        "--thinning-threshold",
        type=int,
        default=5000,
        help="Min token threshold for thinning (default: 5000)",
    )
    parser.add_argument(
        "--single",
        type=str,
        default=None,
        help="Process a single file (e.g. ACONITUM.md) instead of all articles",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default=OUTPUT_DIR,
        help=f"Output directory for JSON results (default: {OUTPUT_DIR})",
    )
    parser.add_argument(
        "--json-only",
        action="store_true",
        help="Only save JSON files locally, skip writing to MongoDB",
    )
    parser.add_argument(
        "--mongo-uri",
        type=str,
        default=None,
        help="MongoDB connection URI (default: MONGO_URI env var)",
    )
    args = parser.parse_args()

    os.makedirs(args.output_dir, exist_ok=True)

    # Collect markdown files
    if args.single:
        # Validate filename: must be a plain filename (no path separators)
        # to prevent path traversal attacks.
        if os.sep in args.single or "/" in args.single or ".." in args.single:
            print(f"Error: --single must be a plain filename (e.g. ACONITUM.md), not a path", file=sys.stderr)
            sys.exit(1)
        md_path = os.path.join(ARTICLES_DIR, args.single)
        # Ensure resolved path is inside ARTICLES_DIR
        if not os.path.realpath(md_path).startswith(os.path.realpath(ARTICLES_DIR)):
            print(f"Error: file must be inside articles/ directory", file=sys.stderr)
            sys.exit(1)
        if not os.path.isfile(md_path):
            print(f"Error: file not found: {md_path}", file=sys.stderr)
            sys.exit(1)
        md_files = [md_path]
    else:
        md_files = sorted(glob.glob(os.path.join(ARTICLES_DIR, "*.md")))

    if not md_files:
        print("No markdown files found in articles/", file=sys.stderr)
        sys.exit(1)

    print(f"Found {len(md_files)} markdown file(s) to process")
    print(f"Model: {args.model}")
    print(f"Summaries: {'yes' if args.with_summaries else 'no'}")
    print(f"Include text: {'yes' if args.with_text else 'no'}")
    print(f"Output dir: {args.output_dir}")

    # Set up MongoDB connection
    mongo_client = None
    mongo_col = None
    if not args.json_only:
        mongo_uri = args.mongo_uri or os.getenv("MONGO_URI")
        if not mongo_uri:
            print("Error: MONGO_URI env var or --mongo-uri required (use --json-only to skip MongoDB)", file=sys.stderr)
            sys.exit(1)
        from pymongo import MongoClient
        mongo_client = MongoClient(mongo_uri)
        mongo_col = mongo_client[MONGO_DB][MONGO_COLLECTION]
        print(f"MongoDB: {MONGO_DB}/{MONGO_COLLECTION}")

    print("=" * 60)

    all_documents = {}
    failed = []

    for i, md_path in enumerate(md_files, 1):
        basename = os.path.basename(md_path)
        name = os.path.splitext(basename)[0]
        print(f"\n[{i}/{len(md_files)}] Processing {basename}...")

        try:
            tree = await build_index_for_file(
                md_path=md_path,
                model=args.model,
                with_summaries=args.with_summaries,
                with_text=args.with_text,
                summary_token_threshold=args.summary_token_threshold,
                thinning=args.thinning,
                thinning_threshold=args.thinning_threshold,
            )

            # Save individual JSON file
            output_file = os.path.join(args.output_dir, f"{name}_structure.json")
            with open(output_file, "w", encoding="utf-8") as f:
                json.dump(tree, f, indent=2, ensure_ascii=False)
            print(f"  -> Saved: {output_file}")

            # Upsert into MongoDB
            if mongo_col is not None:
                mongo_doc = {
                    "_id": name,
                    "docName": tree.get("doc_name", name),
                    "docDescription": tree.get("doc_description", ""),
                    "lineCount": tree.get("line_count", 0),
                    "structure": tree.get("structure", []),
                }
                mongo_col.replace_one({"_id": name}, mongo_doc, upsert=True)
                print(f"  -> Upserted to MongoDB: {name}")

            all_documents[name] = tree

        except Exception as e:
            print(f"  !! Failed: {e}", file=sys.stderr)
            failed.append((basename, str(e)))

    # Save combined index
    combined_file = os.path.join(args.output_dir, "all_articles_index.json")
    with open(combined_file, "w", encoding="utf-8") as f:
        json.dump(all_documents, f, indent=2, ensure_ascii=False)

    if mongo_client:
        mongo_client.close()

    print("\n" + "=" * 60)
    print(f"Done! Processed {len(md_files) - len(failed)}/{len(md_files)} files")
    print(f"Combined index saved to: {combined_file}")

    if failed:
        print(f"\nFailed files ({len(failed)}):")
        for name, err in failed:
            print(f"  - {name}: {err}")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
