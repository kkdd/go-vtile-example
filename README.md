# An example Go app for dynamically serving MapboxGL vector tiles

![](https://cloud.githubusercontent.com/assets/583385/16578797/4cbf4d8a-4251-11e6-9f4c-75820d220405.png)


## Installation

```github.com/golang/protobuf/proto``` is required to be installed (see ```main.go```).

```vector_tile/vector_tile.pb.go``` is produced as follows:

```console
$ cd vector_tile
$ wget https://github.com/mapbox/vector-tile-spec/blob/master/2.1/vector_tile.proto
$ cat vector_tile.proto.diff
--- vector_tile.proto.orig
+++ vector_tile.proto
@@ -1,3 +1,5 @@
+syntax = "proto2";
+
 package vector_tile;
 
 option optimize_for = LITE_RUNTIME;
@@ -52,7 +54,7 @@
                 // number encoded in this message and choose the correct
                 // implementation for this version number before proceeding to
                 // decode other parts of this message.
-                required uint32 version = 15 [ default = 1 ];
+                required uint32 version = 15 [ default = 2 ];
 
                 required string name = 1;
$
$ protoc --go_out=. vector_tile.proto
$ ls -d vector_tile.*
vector_tile.pb.go	vector_tile.proto
$
```

## To run the project

`cd` into the project directory, then run:

```console
$ go run main.go
number of points = 91586
```

In main.go, location is represented as:

```math
loc.x &= \frac{\lambda}{2 \pi} \\
loc.y &= \tan \frac{\phi}{2}
```



## To view the tiles

To view the tiles, you'll need to modify your MapboxGL style to add an additional vector tile layer. Here's an example:

```
var map = new mapboxgl.Map({
  container: 'map',
  zoom: 12.5,
  center: [-122.45, 37.79],
  style: {
    version: 8,
    sources: {},
    layers: []
  },
  hash: false
});

map.on('load', function loaded() {
  map.addSource('custom-go-vector-tile-source', {
      type: 'vector',
      tiles: ['http://localhost:8080/tiles/{z}/{x}/{y}']
  });
  map.addLayer({
    id: 'background',
    type: 'background',
    paint: {
      'background-color': 'white'
    }
  });
  map.addLayer({
      "id": "custom-go-vector-tile-layer",
      "type": "circle",
      "source": "custom-go-vector-tile-source",
      "source-layer": "points",
      paint: {
        'circle-radius': {
          stops: [[8, 0.1], [11, 0.5], [15, 3], [20, 20]]
        },
        'circle-color': '#e74c3c',
        'circle-opacity': 1
      }
  });
});
```

## Data from SFGov.org

"[Street Tree Map | DataSF | City and County of San Francisco](https://data.sfgov.org/City-Infrastructure/Street-Tree-Map/337t-q2b4)"

```console
$ wc -l trees.csv 
   91587
```
