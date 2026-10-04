#!/usr/bin/env python3
"""Sync items from a wishlist JSON file to a running wishlist server.

Default mode creates items that are missing on the server (matched by id).
--update instead changes items that exist on the server but differ from the
file. Purchase status is always taken from the server, so an update never
un-marks something that was bought there.

Dry run by default; pass --apply to write changes.

Admin credentials are read from the environment, never from the command line:
  WISHLIST_ADMIN_USERNAME (default: admin)
  WISHLIST_ADMIN_PASSWORD
"""

import argparse
import base64
import http.cookiejar
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
CSRF_TOKEN_RE = re.compile(r'X-CSRF-Token": "([^"]+)"')


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--base-url", required=True, help="e.g. https://your-site.vercel.app")
    parser.add_argument("--list", default="pedro", dest="slug", help="slug of the list to sync")
    parser.add_argument(
        "--file",
        default=str(REPO_ROOT / "data" / "wishlist.json"),
        help="wishlist JSON to read items from",
    )
    parser.add_argument("--update", action="store_true", help="update changed items instead of creating missing ones")
    parser.add_argument("--apply", action="store_true", help="write changes (default: dry run)")
    args = parser.parse_args()

    password = os.environ.get("WISHLIST_ADMIN_PASSWORD")
    if not password:
        sys.exit("WISHLIST_ADMIN_PASSWORD is not set")
    username = os.environ.get("WISHLIST_ADMIN_USERNAME", "admin")

    base = args.base_url.rstrip("/")
    auth = "Basic " + base64.b64encode(f"{username}:{password}".encode()).decode()
    cookies = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookies))

    def request(method, path, body=None, extra_headers=None):
        headers = {"Authorization": auth, **(extra_headers or {})}
        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(base + path, data=data, method=method, headers=headers)
        try:
            with opener.open(req) as resp:
                return resp.status, resp.read().decode("utf-8")
        except urllib.error.HTTPError as err:
            return err.code, err.read().decode("utf-8", "replace")

    local = json.loads(Path(args.file).read_text(encoding="utf-8"))
    local_list = next((l for l in local["lists"] if l["slug"] == args.slug), None)
    if local_list is None:
        sys.exit(f"list {args.slug!r} not found in {args.file}")

    status, body = request("GET", "/wishlist")
    if status != 200:
        sys.exit(f"GET /wishlist failed: {status} {body[:200]}")
    remote_list = next((l for l in json.loads(body)["lists"] if l["slug"] == args.slug), None)
    if remote_list is None:
        sys.exit(f"list {args.slug!r} does not exist on {base}")
    remote_items = {item["id"]: item for item in remote_list.get("items") or []}

    if args.update:
        plan = []
        for item in local_list["items"]:
            remote = remote_items.get(item["id"])
            if remote is None:
                continue
            item = {**item, "waspurchased": remote.get("waspurchased", False)}
            changes = {k: (remote.get(k), v) for k, v in item.items() if remote.get(k) != v}
            if changes:
                plan.append((item, changes))
        print(f"{len(plan)} items to update")
        for item, changes in plan:
            print(f"  ~ {item['title']}")
            for field, (old, new) in changes.items():
                print(f"      {field}: {old!r} -> {new!r}")
        method = "POST"
        items_to_send = [item for item, _ in plan]
    else:
        missing = [item for item in local_list["items"] if item["id"] not in remote_items]
        print(f"server has {len(remote_items)} items; file has {len(local_list['items'])}; {len(missing)} to create")
        for item in missing:
            print(f"  + {item['title']}  [{item['itemtype']}]")
        method = "PUT"
        items_to_send = missing

    if not args.apply:
        print("dry run: nothing changed (pass --apply to write)")
        return
    if not items_to_send:
        return

    status, page = request("GET", f"/wishlist/{args.slug}")
    token_match = CSRF_TOKEN_RE.search(page) if status == 200 else None
    if token_match is None:
        sys.exit(f"could not get a CSRF token from /wishlist/{args.slug} (status {status})")
    token = token_match.group(1)

    for item in items_to_send:
        status, body = request(
            method, f"/wishlist/{args.slug}/wishitem", item, {"X-CSRF-Token": token}
        )
        if status != 200:
            sys.exit(f"stopped at {item['title']!r}: {status} {body[:200]} (safe to re-run)")
        print(f"  {'updated' if args.update else 'created'} {item['title']}")


if __name__ == "__main__":
    main()
