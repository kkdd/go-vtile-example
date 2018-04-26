package main

import (
	"os"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"github.com/golang/protobuf/proto"
	"./vector_tile"
)

func cmdEnc(id uint32, count uint32) uint32 {
	return (id & 0x7) | (count << 3)
}

func moveTo(count uint32) uint32 {
	return cmdEnc(1, count)
}

func lineTo(count uint32) uint32 {
	return cmdEnc(2, count)
}

func closePath(count uint32) uint32 {
	return cmdEnc(7, count)
}

func paramEnc(value int32) int32 {
	return (value << 1) ^ (value >> 31)
}

func createTileWithPoints(points []XY, bounds XYZ) ([]byte, error) {
	layerName := "points"
	var layerVersion = vector_tile.Default_Tile_Layer_Version
	featureType := vector_tile.Tile_POINT
	var extent = vector_tile.Default_Tile_Layer_Extent
	var geometry []uint32
	geometry = append(geometry, 0)  // dummy
	var pX int32
	var pY int32
	x, y := tileToBoundingBox(bounds)
	for _, point := range points {
		if point.x >= x[0] && point.x < x[1] && point.y >= y[0] && point.y < y[1] {
			p := locToTileXY(point, bounds)
			deltaX := int32(float64(extent)*p.x+0.5) - pX
			deltaY := int32(float64(extent)*p.y+0.5) - pY
			geometry = append(geometry, uint32(paramEnc(deltaX)))
			geometry = append(geometry, uint32(paramEnc(deltaY)))
			pX = pX + deltaX
			pY = pY + deltaY
		}
	}
	geometry[0] = moveTo((uint32(len(geometry))-1)/2)
	tile := &vector_tile.Tile{}
	tile.Layers = []*vector_tile.Tile_Layer{
		&vector_tile.Tile_Layer{
			Version: &layerVersion,
			Name:	&layerName,
			Extent:  &extent,
			Features: []*vector_tile.Tile_Feature{
				&vector_tile.Tile_Feature{
					Tags:	 []uint32{},
					Type:	 &featureType,
					Geometry: geometry,
				},
			},
		},
	}
	return proto.Marshal(tile)
}

// return loc: Braun projection
func lonLatToLoc(lonLat XY) (XY) {
	var loc XY
	loc.x = lonLat.x/360
	loc.y = math.Tan(lonLat.y/360 * math.Pi)  // Braun projection
	return loc
}

func locToLonLat(loc XY) (XY) {
	var lonLat XY
	lonLat.x = loc.x * 360
	lonLat.y = math.Atan(loc.y) * 360/math.Pi  // inverse Braun projection
	return lonLat
}

// relative position in a tile
func locToTileXY(loc XY, tile XYZ) (XY) {
	pos := loc
	pos.y = math.Log((1+pos.y)/(1-pos.y))/math.Pi/2  // web mercator
	pos.x = ( pos.x + 0.5) * tile.z - tile.x
	pos.y = (-pos.y + 0.5) * tile.z - tile.y
	return pos
}

func tileToLoc(tile XYZ) (XY) {
	var loc XY
	loc.x =   tile.x / tile.z - 0.5
	loc.y = -(tile.y / tile.z - 0.5)
	loc.y = 1 - 2/(math.Exp(loc.y*math.Pi*2)+1)  // inverse web mercator
	return loc
}

func tileToBoundingBox(tile XYZ) ([]float64, []float64) {
	upper := tileToLoc(tile)
	lower := tileToLoc(XYZ{x: tile.x, y: tile.y+1, z: tile.z})
	return []float64{upper.x, upper.x + 1/tile.z}, []float64{lower.y, upper.y}
}

const RE = 6378137.0  // GRS80
const FE = 1/298.257223563  // IS-GPS
const E2 = FE * (2 - FE)

//  geographic distance between two points
//  inputs: p = lonLatToLoc(lonLat1), q = lonLatToLoc(lonLat2)
func distance(p XY, q XY) (float64) {
	y2 := square((p.y + q.y) / 2)
	coslat := (1 - y2) / (1 + y2)
	w2 := 1 / (1 - E2 * (1 - coslat * coslat))
	dx := (p.x - q.x) * coslat
	dy := (p.y - q.y) * 2 / (1 + y2) * w2 * (1 - E2)
	return math.Sqrt(hypotSquared(dx, dy) * w2) * 2 * math.Pi * RE
}

func square(x float64) (float64) {
	return x * x
}

func hypotSquared(x float64, y float64) (float64) {
	return x * x + y * y
}

// Takes a string of the form `<z>/<x>/<y>` (for example, 1/2/3) and returns
// the individual uint32 values for x, y, and z if there was no error.
// Otherwise, err is set to a non `nil` value and x, y, z are set to 0.
func pathToTile(path string) (XYZ, error) {
	xyzReg := regexp.MustCompile("(?P<z>[0-9]+)/(?P<x>[0-9]+)/(?P<y>[0-9]+)")
	matches := xyzReg.FindStringSubmatch(path)
	if len(matches) == 0 {
		return XYZ{}, errors.New("Unable to parse path as tile")
	}
	x, err := strconv.ParseUint(matches[2], 10, 32)
	if err != nil {
		return XYZ{}, err
	}
	y, err := strconv.ParseUint(matches[3], 10, 32)
	if err != nil {
		return XYZ{}, err
	}
	z, err := strconv.ParseUint(matches[1], 10, 32)
	if err != nil {
		return XYZ{}, err
	}
	return XYZ{x: float64(x), y: float64(y), z: math.Pow(2, float64(z))}, nil
}

// A XYZ is a struct that holds tile's coordinates and zoom scale.
type XYZ struct {
	x float64
	y float64
	z float64
}

// A XY is a struct that holds a geographic location.
type XY struct {
	x float64
	y float64
}

// Tree a struct holder for tree information.
type Tree struct {
	lonlat XY
	species string
}

// trees.csv: TreeID,qLegalStatus,qSpecies,qAddress,SiteOrder,qSiteInfo,PlantType,qCaretaker,qCareAssistant,PlantDate,DBH,PlotSize,PermitNotes,XCoord,YCoord,Latitude,Longitude,Location
const SPECIES = 2
const LATITUDE = 15
const LONGITUDE = 16

func loadTrees() []Tree {
	content, err := ioutil.ReadFile("./trees.csv")
	if err != nil {
		log.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(content[:])))
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	var trees []Tree
	for _, record := range records[1:] {
		species := record[SPECIES]
		lon, _ := strconv.ParseFloat(record[LONGITUDE], 64)
		lat, _ := strconv.ParseFloat(record[LATITUDE], 64)
		trees = append(trees, Tree{lonlat: XY{x: lon, y: lat}, species: species})
	}
	return trees
}

func main() {
	trees := loadTrees()
	points := make([]XY, len(trees), len(trees))
	for i, tree := range trees {
		points[i] = lonLatToLoc(tree.lonlat)
	}
	fmt.Println("number of points =", len(points))
	mux := http.NewServeMux()
	// Handle requests for urls of the form `/tiles/{z}/{x}/{y}` and returns
	// the vector tile for the even tile x, y, and z coordinates.
	tileBase := "/tiles/"
	mux.HandleFunc(tileBase, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("url: %s", r.URL.Path)
		tile, err := pathToTile(r.URL.Path[len(tileBase):])
		if err != nil {
			http.Error(w, "Invalid tile url", 400)
			return
		}
		data, err := createTileWithPoints(points, tile)
		if err != nil {
			log.Fatal("error generating tile", err)
		}
		// All this APi to be requests from other domains.
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Write(data)
	})
	log.Fatal(http.ListenAndServe(":8080", mux))
}
