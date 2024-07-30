package main

import (
	"fmt"

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

func (tile *Tile) drawTile(screenPos rl.Vector2) {
	color, _ := getColorForTerrain(tile)
	rl.DrawRectangle(int32(screenPos.X), int32(screenPos.Y), int32(gameSettings.TILE_SIZE.X), int32(gameSettings.TILE_SIZE.Y), color)
}

var gameSettings = struct {
	TILE_SIZE   Vector2Int32
	CLEAR_COLOR rl.Color
	RNG_SEED    int64
	MAP_SCALAR  rl.Vector2
	CHUNK_SIZE  Vector2Int32
	WATER_LEVEL float32
}{
	Vector2Int32{5, 5},
	rl.NewColor(255, 255, 255, 255),
	256,
	rl.NewVector2(100, 100),
	Vector2Int32{25, 25},
	0.4,
}

type Vector2Int32 struct {
	X, Y int32
}

func (v *Vector2Int32) asRlVector2() rl.Vector2 {
	return rl.NewVector2(float32(v.X), float32(v.Y))
}

// TODO implement chunking and infinite rendering

type Chunk struct {
	tiles Matrix[Tile]
}

type ChunkMap struct {
	coordToChunk map[Vector2Int32]*Chunk
}

func generateChunk(chunkCoordinate Vector2Int32) *Chunk {
	tiles := MakeMatrix[Tile](int(gameSettings.CHUNK_SIZE.X), int(gameSettings.CHUNK_SIZE.Y))

	for y := int32(0); y < int32(tiles.h); y++ {
		for x := int32(0); x < int32(tiles.w); x++ {
			height := getHeight(Vector2Int32{
				chunkCoordinate.X*int32(gameSettings.CHUNK_SIZE.X) + x,
				chunkCoordinate.Y*int32(gameSettings.CHUNK_SIZE.Y) + y,
			})
			tiles.Set(int(x), int(y), Tile{
				terrain:     getTerrain(height),
				waterlogged: height < gameSettings.WATER_LEVEL,
				height:      height,
			})
		}
	}
	return &Chunk{tiles}
}

func (c *ChunkMap) getChunk(chunkCoordinate Vector2Int32) *Chunk {
	chunk, ok := c.coordToChunk[chunkCoordinate]
	if !ok {
		chunk = generateChunk(chunkCoordinate)
		c.coordToChunk[chunkCoordinate] = chunk
	}
	return chunk
}

func (cm *ChunkMap) getTile(tileCoord Vector2Int32) Tile {

	chunkCoord := getChunkCoords(tileCoord)
	coordInChunk := Vector2Int32{
		X: mod(tileCoord.X, gameSettings.CHUNK_SIZE.X),
		Y: mod(tileCoord.Y, gameSettings.CHUNK_SIZE.Y),
	}

	// // Make chunk edges magenta
	// if coordInChunk.X == 0 || coordInChunk.Y == 0 {
	// 	return Tile{terrain: UndefinedTerrain}
	// }

	chunk := cm.getChunk(chunkCoord)
	return chunk.tiles.At(
		int(coordInChunk.X),
		int(coordInChunk.Y),
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
func getHeight(coordinate Vector2Int32) float32 {
	pNoise, _ := noise.New(noise.Perlin, gameSettings.RNG_SEED)
	floatCoord := coordinate.asRlVector2()
	var height float32 = pNoise.Eval32(
		floatCoord.X/gameSettings.MAP_SCALAR.X,
		floatCoord.Y/gameSettings.MAP_SCALAR.Y,
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

func (c *Camera) getTileCoords() Vector2Int32 {
	return Vector2Int32{
		X: int32(c.center.X / float32(gameSettings.TILE_SIZE.X)),
		Y: int32(c.center.Y / float32(gameSettings.TILE_SIZE.Y)),
	}
}

// -2, -1
func getChunkCoords(tileCoord Vector2Int32) Vector2Int32 {
	res := Vector2Int32{
		X: tileCoord.X / int32(gameSettings.CHUNK_SIZE.X),
		Y: tileCoord.Y / int32(gameSettings.CHUNK_SIZE.Y),
	}
	if tileCoord.X < 0 {
		res.X -= 1
	}
	if tileCoord.Y < 0 {
		res.Y -= 1
	}
	return res
}

type Game struct {
	windowSize Vector2Int32
	camera     Camera
	chunkMap   ChunkMap
}

func (g *Game) toScreenCoord(worldCoord rl.Vector2) rl.Vector2 {
	return rl.NewVector2(
		worldCoord.X-g.camera.center.X+float32(g.windowSize.X/2),
		worldCoord.Y-g.camera.center.Y+float32(g.windowSize.Y/2),
	)
}

func (g *Game) toWorldCoord(screenCoord rl.Vector2) rl.Vector2 {
	return rl.NewVector2(
		screenCoord.X+g.camera.center.X-float32(g.windowSize.X/2),
		screenCoord.Y+g.camera.center.Y-float32(g.windowSize.Y/2),
	)
}

func (g *Game) toTileCoord(worldCoord rl.Vector2) Vector2Int32 {
	res := Vector2Int32{
		X: int32(worldCoord.X) / gameSettings.TILE_SIZE.X,
		Y: int32(worldCoord.Y) / gameSettings.TILE_SIZE.Y,
	}
	if worldCoord.X < 0 {
		res.X -= 1
	}
	if worldCoord.Y < 0 {
		res.Y -= 1
	}
	return res
}

func (g *Game) draw() {
	tileSize := gameSettings.TILE_SIZE
	for y := int32(0); y < int32(g.windowSize.Y/tileSize.Y); y++ {
		for x := int32(0); x < int32(g.windowSize.X/tileSize.X); x++ {
			worldCoord := g.toWorldCoord(rl.NewVector2(float32(x*tileSize.X), float32(y*tileSize.Y)))
			tileCoord := g.toTileCoord(worldCoord)
			tile := g.chunkMap.getTile(tileCoord)
			tile.drawTile(g.toScreenCoord(worldCoord))

			// DEBUG
			if tileCoord.X == 0 && tileCoord.Y == 0 {
				t := Tile{terrain: UndefinedTerrain}
				t.drawTile(g.toScreenCoord(worldCoord))
			}
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

	// q to zoom in, e to zoom out
	if rl.IsKeyDown(rl.KeyQ) {
		gameSettings.TILE_SIZE.X += 1
		gameSettings.TILE_SIZE.Y += 1
	}
	if rl.IsKeyDown(rl.KeyE) {
		gameSettings.TILE_SIZE.X = max(1, gameSettings.TILE_SIZE.X-1)
		gameSettings.TILE_SIZE.Y = max(1, gameSettings.TILE_SIZE.Y-1)
	}

}

func main() {
	game := Game{
		Vector2Int32{800, 600},
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
