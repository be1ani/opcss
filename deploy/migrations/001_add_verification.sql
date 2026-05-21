-- Migration 001: add per-file integrity verification columns
--
-- verification_status: 'ok' | 'corrupted' — NULL until first verification run
-- file_checksum: SHA-256 of concatenated per-chunk hex digests (merkle-style)
-- verified_at: timestamp of the most recent verification pass

ALTER TABLE files
    ADD COLUMN verified_at         TIMESTAMPTZ,
    ADD COLUMN file_checksum       TEXT,
    ADD COLUMN verification_status TEXT;
