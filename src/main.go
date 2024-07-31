package main

import (
	"fmt"
	"math"

	noise "github.com/KEINOS/go-noise"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type TerrainType int

const (
	UndefinedTerrain TerrainType = iota
	Sand             TerrainType = iota
	Gravel           TerrainType = iota
	Dirt             TerrainType = iota
	LowGrass         TerrainType = iota
	HighGrass        TerrainType = iota
	Forest           TerrainType = iota
	Mountain         TerrainType = iota
)

type Tile struct {
	terrain     TerrainType
	waterlogged bool
	height      float32
}

// maps from terrain type to color
var terrianTypeToColor = map[TerrainType]rl.Color{
	UndefinedTerrain: rl.Magenta,
	Sand:             rl.Yellow,
	Gravel:           rl.Gray,
	Dirt:             rl.Brown,
	LowGrass:         rl.Lime,
	HighGrass:        rl.Green,
	Forest:           rl.DarkGreen,
	Mountain:         rl.White,
}

type Integer interface {
	int | int16 | int32 | int64
}

func mod[T Integer](a, b T) T {
	return (a%b + b) % b
}

// tries to return color, but if it fails returns magenta and error
func getColorForTerrain(tile *Tile) (rl.Color, error) {
	if tile.waterlogged {
		return rl.Blue, nil
	}

	color, ok := terrianTypeToColor[tile.terrain]
	if !ok {
		return rl.Magenta, fmt.Errorf("no color for terrain type %v", tile.terrain)
	}
	return color, nil
}

var gameSettings = struct {
	TILE_SIZE   rl.Vector2
	CLEAR_COLOR rl.Color
	RNG_SEED    int64
	MAP_SCALAR  rl.Vector2
	CHUNK_SIZE  rl.Vector2
	WATER_LEVEL float32
}{
	rl.NewVector2(5, 5),
	rl.NewColor(255, 255, 255, 255),
	256,
	rl.NewVector2(50, 50),
	rl.NewVector2(25, 25),
	0.4,
}

func (tile *Tile) drawTile(screenPos rl.Vector2) {
	color, _ := getColorForTerrain(tile)
	rl.DrawRectangle(
		int32(screenPos.X),
		int32(screenPos.Y),
		int32(gameSettings.TILE_SIZE.X),
		int32(gameSettings.TILE_SIZE.Y),
		color,
	)
}

type Coordinate struct {
	X, Y int32
}

func rlVector2ToCoordinate(v *rl.Vector2) Coordinate {
	return Coordinate{floor32Int(v.X), floor32Int(v.Y)}
}
func floor32Int(x float32) int32 {
	return int32(math.Floor(float64(x)))
}

// TODO implement chunking and infinite rendering

type Chunk struct {
	tiles Matrix[Tile]
}

type ChunkMap struct {
	coordToChunk map[Coordinate]*Chunk
}

func generateChunk(chunkCoordinate rl.Vector2) *Chunk {
	tiles := MakeMatrix[Tile](int(gameSettings.CHUNK_SIZE.X), int(gameSettings.CHUNK_SIZE.Y))

	for y := float32(0); y < float32(tiles.h); y += 1 {
		for x := float32(0); x < float32(tiles.w); x += 1 {
			worldCoord := chunkCoordinate
			scaleVec2(&worldCoord, gameSettings.CHUNK_SIZE.X, gameSettings.CHUNK_SIZE.Y)
			translateVec2(&worldCoord, x, y)
			height := getHeight(worldCoord)

			tiles.Set(int(x), int(y), Tile{
				terrain:     getTerrain(height),
				waterlogged: height < gameSettings.WATER_LEVEL,
				height:      height,
			})
		}
	}
	return &Chunk{tiles}
}

func (c *ChunkMap) getChunk(chunkCoordinate Coordinate) *Chunk {
	chunk, ok := c.coordToChunk[chunkCoordinate]
	if !ok {
		chunk = generateChunk(chunkCoordinate)
		c.coordToChunk[chunkCoordinate] = chunk
	}
	return chunk
}

func (cm *ChunkMap) getTile(tileCoord rl.Vector2) Tile {
	chunkCoord := &tileCoord
	toChunkCoords(chunkCoord)

	coordInChunkX := mod(int32(tileCoord.X), int32(gameSettings.CHUNK_SIZE.X))
	coordInChunkY := mod(int32(tileCoord.Y), int32(gameSettings.CHUNK_SIZE.Y))

	// // Make chunk edges magenta
	// if coordInChunk.X == 0 || coordInChunk.Y == 0 {
	// 	return Tile{terrain: UndefinedTerrain}
	// }

	chunk := cm.getChunk(rlVector2ToCoordinate(chunkCoord))
	return chunk.tiles.At(
		int(coordInChunkX),
		int(coordInChunkY),
	)
}

type Matrix[T any] struct {
	w, h int
	data []T
}

func MakeMatrix[T any](w, h int) Matrix[T] { return Matrix[T]{w, h, make([]T, w*h)} }
func (m Matrix[T]) At(x, y int) T          { return m.data[y*m.w+x] }
func (m Matrix[T]) Set(x, y int, t T)      { m.data[y*m.w+x] = t }

// returns in range [0, 1]
// expects x, y to be in tile coordinates
func getHeight(coordinate *rl.Vector2) float32 {
	pNoise, _ := noise.New(noise.Perlin, gameSettings.RNG_SEED)
	var height float32 = pNoise.Eval32(
		coordinate.X/gameSettings.MAP_SCALAR.X,
		coordinate.Y/gameSettings.MAP_SCALAR.Y,
	)
	return (height + 1) / 2
}

// Keep this in order of increasing height
var heightBoundaryToTile = []struct {
	UpperBound float32
	Terrain    TerrainType
}{
	{0.42, Sand},
	{0.45, Gravel},
	{0.47, Dirt},
	{0.55, LowGrass},
	{0.6, HighGrass},
	{0.78, Forest},
	{1.0, Mountain},
}

func getTerrain(height float32) TerrainType {
	for _, tileAtBound := range heightBoundaryToTile {
		if height < tileAtBound.UpperBound {
			return tileAtBound.Terrain
		}
	}

	return UndefinedTerrain
}

type Camera struct {
	center rl.Vector2
}

func toChunkCoords(tileCoord *rl.Vector2) {
	floorDivVec2(tileCoord, gameSettings.CHUNK_SIZE.X, gameSettings.CHUNK_SIZE.Y)
}

type Game struct {
	windowSize rl.Vector2
	camera     Camera
	chunkMap   ChunkMap
}

func toTileCoord(worldCoord *rl.Vector2) {
	floorDivVec2(worldCoord, gameSettings.TILE_SIZE.X, gameSettings.TILE_SIZE.Y)
}

func (g *Game) toWorldCoord(screenCoord *rl.Vector2) {
	translateVec2(screenCoord, g.windowSize.X/2, g.windowSize.Y/2)
	translateVec2(screenCoord, -g.camera.center.X, -g.camera.center.Y)
}

func (g *Game) toScreenCoord(worldCoord *rl.Vector2) {
	translateVec2(worldCoord, -g.camera.center.X, -g.camera.center.Y)
	translateVec2(worldCoord, g.windowSize.X/2, g.windowSize.Y/2)
}

func translateVec2(v *rl.Vector2, x float32, y float32) {
	v.X += x
	v.Y += y
}

func floorDivVec2(v *rl.Vector2, x, y float32) {
	scaleVec2(v, 1.0/x, 1.0/y)
	floorVec2(v)
}

func scaleVec2(v *rl.Vector2, x, y float32) {
	v.X *= x
	v.Y *= y
}

func floorVec2(v *rl.Vector2) {
	v.X = float32(math.Floor(float64(v.X)))
	v.Y = float32(math.Floor(float64(v.Y)))
}

func (g *Game) draw() {
	tileSize := gameSettings.TILE_SIZE
	for y := float32(0); y < g.windowSize.Y/tileSize.Y; y += 1 {
		for x := float32(0); x < g.windowSize.X/tileSize.X; x += 1 {
			v := rl.NewVector2(
				x*tileSize.X,
				y*tileSize.Y,
			)

			g.toWorldCoord(&v)
			toTileCoord(&v)
			g.chunkMap.getTile(v).drawTile(v)
			// tile := g.chunkMap.getTile(NewVector2Int32FromRl(&v))

			// v_world := rl.NewVector2(
			// 	v_screen.X+g.camera.center.X,
			// 	v_screen.Y+g.camera.center.Y,
			// )
			// v_tile :=

			// tile := g.chunkMap.getTile(v_tile)
			// tile.drawTile(g.toScreenCoord(v_world))

		}
	}

}

func (g *Game) handleInput() {
	var moveSpeed float32 = 5.25
	if rl.IsKeyDown(rl.KeyW) {
		g.camera.center.Y -= moveSpeed
	}
	if rl.IsKeyDown(rl.KeyS) {
		g.camera.center.Y += moveSpeed
	}
	if rl.IsKeyDown(rl.KeyA) {
		g.camera.center.X -= moveSpeed
	}
	if rl.IsKeyDown(rl.KeyD) {
		g.camera.center.X += moveSpeed
	}

}

func main() {
	game := Game{
		Vector2Int32{1080, 720},
		Camera{rl.NewVector2(0, 0)},
		ChunkMap{make(map[Vector2Int32]*Chunk)},
	}

	rl.InitWindow(int32(game.windowSize.X),
		int32(game.windowSize.Y),
		"Another grid game",
	)
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(gameSettings.CLEAR_COLOR)
		game.draw()
		game.handleInput()
		rl.EndDrawing()
	}
}
