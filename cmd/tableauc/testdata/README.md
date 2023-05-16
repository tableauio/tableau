# Test

## test merger/scatter

Auto find the primary workbook if only input a secondary workbook.

1. `cd cmd/tableauc`, run `./tableauc -c testdata/config.yaml`
2. merger test: run `./tableauc -c testdata/config.yaml testdata/csv/Merger3#MergerZone.csv`
3. scatter test: run `./tableauc -c testdata/config.yaml testdata/csv/Scatter3#ScatterZone.csv`

## test subdir rewrites

1. `cd cmd/tableauc`, run `./tableauc -c testdata/config.yaml`
2. `cd cmd/tableauc/testdata`:
   1. merger test:
       - run `./../tableauc -c config_subdir_rewrites.yaml csv/Merger1#MergerZone.csv`
       - run `./../tableauc -c config_subdir_rewrites.yaml csv/Merger3#MergerZone.csv`
   2. scatter test:
       - run `./../tableauc -c config_subdir_rewrites.yaml csv/Scatter1#ScatterZone.csv`
       - run `./../tableauc -c config_subdir_rewrites.yaml csv/Scatter3#ScatterZone.csv`
