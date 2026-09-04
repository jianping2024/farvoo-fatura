-- Terminal lifecycle + default print station (per LAN terminal + Agent local)

ALTER TABLE fiscal_terminals ADD COLUMN default_station_id TEXT;
ALTER TABLE taxpayer_settings ADD COLUMN local_default_station_id TEXT;
