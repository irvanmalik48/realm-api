import os
import glob
import re
import subprocess
import json

posts_dir = "/home/lappland/Projects/realm-reference/posts"
files = sorted(glob.glob(os.path.join(posts_dir, "*.mdx")) + glob.glob(os.path.join(posts_dir, "*.md")))

print(f"Found {len(files)} post files to seed.")

for f in files:
    slug = os.path.splitext(os.path.basename(f))[0]
    with open(f, "r", encoding="utf-8") as fh:
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

    created_at = "2026-01-01"
    created_match = re.search(r'createdAt:\s*["\']?(.*?)["\']?$', frontmatter_raw, re.M)
    if created_match:
        created_at = created_match.group(1).strip()

    updated_at = created_at
    updated_match = re.search(r'updatedAt:\s*["\']?(.*?)["\']?$', frontmatter_raw, re.M)
    if updated_match:
        updated_at = updated_match.group(1).strip()

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

    print(f"Processing: {slug} | Title: {title} | Words: {word_count} | Tags: {tags}")

    tags_array_literal = "{" + ",".join([f'"{t}"' for t in tags]) + "}"

    sql = """
    INSERT INTO posts (slug, title, description, content, tags, reading_time, is_published, published_at, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5::text[], $6, true, $7::timestamptz, $8::timestamptz, $9::timestamptz)
    ON CONFLICT (slug) DO UPDATE SET
        title = EXCLUDED.title,
        description = EXCLUDED.description,
        content = EXCLUDED.content,
        tags = EXCLUDED.tags,
        reading_time = EXCLUDED.reading_time,
        is_published = EXCLUDED.is_published,
        published_at = EXCLUDED.published_at,
        updated_at = EXCLUDED.updated_at;
    """

    # Run psql with escaped arguments
    import tempfile
    with tempfile.NamedTemporaryFile("w", suffix=".sql", delete=False) as tf:
        escaped_content = content.replace("'", "''")
        escaped_title = title.replace("'", "''")
        escaped_desc = description.replace("'", "''")
        tags_sql = "ARRAY[" + ",".join([f"'{t}'" for t in tags]) + "]::TEXT[]" if tags else "'{}'::TEXT[]"
        
        tf.write(f"""
        INSERT INTO posts (slug, title, description, content, tags, reading_time, is_published, published_at, created_at, updated_at)
        VALUES (
            '{slug}',
            '{escaped_title}',
            '{escaped_desc}',
            '{escaped_content}',
            {tags_sql},
            '{reading_time}',
            true,
            '{created_at}'::timestamptz,
            '{created_at}'::timestamptz,
            '{updated_at}'::timestamptz
        )
        ON CONFLICT (slug) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            content = EXCLUDED.content,
            tags = EXCLUDED.tags,
            reading_time = EXCLUDED.reading_time,
            is_published = EXCLUDED.is_published,
            published_at = EXCLUDED.published_at,
            updated_at = EXCLUDED.updated_at;
        """)
        tf_name = tf.name

    res = subprocess.run(["psql", "-U", "postgres", "-d", "realm", "-f", tf_name], capture_output=True, text=True)
    os.unlink(tf_name)
    if res.returncode != 0:
        print(f"Error seeding {slug}:", res.stderr)
    else:
        print(f"Successfully seeded: {slug}")

print("Seeding completed!")
