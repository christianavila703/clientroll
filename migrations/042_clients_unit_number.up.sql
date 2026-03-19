-- Add separate unit number field to client addresses.
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS unit_number VARCHAR(32);
