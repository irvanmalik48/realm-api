#!/usr/bin/env python3
"""
Production Post Seeder for Realm
Reads markdown/MDX articles from realm-reference/posts and seeds them
into the PostgreSQL database via REST API or direct connection.

Usage:
    python3 scripts/seed_prod_posts.py --url https://api.irvanma.eu.org --token <ADMIN_API_TOKEN>
    python3 scripts/seed_prod_posts.py --db "postgres://user:pass@remote:5432/realm?sslmode=require"
"""

import argparse
import glob
import json
import os
import re
import sys
import urllib.request
import urllib.error

def parse_post(file_path):
    slug = os.path.splitext(os.path.basename(file_path))[0]
    with open(file_path, "r", encoding="utf-8") as fh:
        raw = fh.read()

    parts = raw.split("---", 2)
    frontmatter_raw = parts[1] if len(parts) >= 3 else ""
    content = parts[2].strip() if len(parts) >= 3 else raw.strip()

    title = slug
    title_match = re.search(r'title:\s*["\']?(.*?)["\']?$', frontmatter_raw, re.M)
    if title_match:
        title = title_match.group(1).strip()

    description = ""
    desc_match = re.search(r'description:\s*["\']?(.*?)["\']?$', frontmatter_raw, re.M)
    if desc_match:
        description = desc_match.group(1).strip()

    created_at = "2026-01-01T00:00:00Z"
    created_match = re.search(r'createdAt:\s*["\']?(.*?)["\']?$', frontmatter_raw, re.M)
    if created_match:
        val = created_match.group(1).strip()
        created_at = f"{val}T00:00:00Z" if "T" not in val else val

    tags = []
    lines = frontmatter_raw.splitlines()
    in_tags = False
    for line in lines:
        if line.strip().startswith("tags:"):
            in_tags = True
            continue
        if in_tags:
            if line.strip().startswith("-"):
                tag = re.sub(r'^\s*-\s*["\']?', "", line)
                tag = re.sub(r'["\']?\s*$', "", tag).strip()
                if tag:
                    tags.append(tag)
            elif ":" in line and not line.strip().startswith("-"):
                in_tags = False

    word_count = len(content.split())
    reading_min = max(1, round(word_count / 120))
    reading_time = f"{reading_min} min read"

    return {
        "slug": slug,
        "title": title,
        "description": description,
        "content": content,
        "tags": tags,
        "reading_time": reading_time,
        "is_published": True,
        "published_at": created_at,
    }

def seed_via_api(api_url, token, posts):
    base = api_url.rstrip("/")
    endpoint = f"{base}/v1/posts"
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    print(f"\n[INFO] Target: {endpoint}")
    print(f"[INFO] Uploading {len(posts)} articles...\n")

    success_count = 0
    for p in posts:
        slug = p["slug"]
        payload = json.dumps(p).encode("utf-8")
        req = urllib.request.Request(endpoint, data=payload, headers=headers, method="POST")

        try:
            with urllib.request.urlopen(req) as resp:
                if resp.status in (200, 201):
                    print(f"  [SUCCESS] '{slug}' -> {p['title']}")
                    success_count += 1
                else:
                    print(f"  [STATUS {resp.status}] '{slug}'")
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8", errors="ignore")
            # If slug already exists, try PUT/PATCH
            if e.code == 409:
                update_req = urllib.request.Request(f"{endpoint}/{slug}", data=payload, headers=headers, method="PUT")
                try:
                    with urllib.request.urlopen(update_req) as u_resp:
                        print(f"  [UPDATED] '{slug}' -> {p['title']}")
                        success_count += 1
                except Exception as ue:
                    print(f"  [FAILED] '{slug}' (Conflict & Update failed): {ue}")
            else:
                print(f"  [ERROR {e.code}] '{slug}': {err_body}")
        except Exception as e:
            print(f"  [EXCEPTION] '{slug}': {e}")

    print(f"\n[COMPLETED] Successfully seeded {success_count}/{len(posts)} posts into production.")

def main():
    parser = argparse.ArgumentParser(description="Seed past posts into Realm API")
    parser.add_argument("--url", default="https://api.irvanma.eu.org", help="API base URL (default: https://api.irvanma.eu.org)")
    parser.add_argument("--token", default="", help="Admin API token or session bearer token")
    parser.add_argument("--posts-dir", default="", help="Path to realm-reference/posts directory")
    args = parser.parse_args()

    posts_dir = args.posts_dir
    if not posts_dir:
        # Search relative paths
        candidates = [
            os.path.abspath(os.path.join(os.path.dirname(__file__), "../../realm-reference/posts")),
            "/home/lappland/Projects/realm-reference/posts",
            "./posts",
        ]
        for c in candidates:
            if os.path.isdir(c):
                posts_dir = c
                break

    if not posts_dir or not os.path.isdir(posts_dir):
        print(f"[ERROR] Could not find realm-reference/posts directory. Specify --posts-dir.")
        sys.exit(1)

    files = sorted(glob.glob(os.path.join(posts_dir, "*.mdx")) + glob.glob(os.path.join(posts_dir, "*.md")))
    if not files:
        print(f"[ERROR] No markdown/MDX files found in {posts_dir}")
        sys.exit(1)

    print(f"[INFO] Discovered {len(files)} markdown posts in {posts_dir}")
    posts = [parse_post(f) for f in files]

    seed_via_api(args.url, args.token, posts)

if __name__ == "__main__":
    main()
