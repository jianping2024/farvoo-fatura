-- local_print_jobs: station for shared Agent printToTarget (tcp|winspool)
ALTER TABLE local_print_jobs ADD COLUMN station_id TEXT;
