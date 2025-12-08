# Overview

This project keeps calendar events, project management boards, and markdown notes in sync.

This branch tracks the rewrite of the project to be much more robust and flexible.

The sync shall be such that changes at any endpoint also update the other services.

# Requirements

- We only need to support a single user
- We need to be able to successfully recover after downtime (server is shut down for maintenance, etc)
    - This means that we shouldn't use webhooks to listen to external source changes

# Design

From this point on, markdown files, events, and tasks will be referred to simply as an "event".

The server will maintain an internal DB of all events. This is because, after the first sync, updates will only be
incremental.

Clients will all implement a shared interface so that they are plug and playable.

## Database Design

```postgresql
CREATE TABLE IF NOT EXISTS sync_items
(
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    gcal_id         TEXT UNIQUE,
    yt_id           TEXT UNIQUE,
    gcal_updated_at TIMESTAMP,
    yt_updated_at   TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_items_gcal_id ON sync_items (gcal_id);
CREATE INDEX IF NOT EXISTS idx_sync_items_yt_id ON sync_items (yt_id);
```

## Handling changes (generally)

At the top of the hour, the syncing service will run and check for changes across all services. Polling is less
efficient than listening for changes, however is a better fit for the requirement that this service must be able to
recover after imposed downtime.

There are 3 scenarios:

- YT changed task
- GC changed task
- Both changed task
  - The changes will be concatenated, thus allowing the user to resolve the conflict in their preferred editor.

## Handling changes in Youtrack (YT)

YT does not surface deletions very well. Thus, every night this service will query YT for each event. If an event is
found to not exist, it will be marked for deletion.

## Handling changes in Google Calendar (GC)

For simplicity, I will be using the Google Calendar API. This results in vendor lock in (compared to CalDAV), but the
syncing method is simpler; I just need to provide a sync token.

## Handling changes in Markdown

My markdown files are hosted in Nextcloud as well as a Git repository.

Nextcloud does not seem to expose an API to easily work with changes. I will thus be using a local git repository to
detect changes.

Markdown events will be identified using the filename, which will match the event name in the internal DB.