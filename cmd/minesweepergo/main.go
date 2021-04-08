package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type GameBoard struct {
	Tiles []*Tile
	W, H  int
}

func (g GameBoard) At(x, y int) *Tile {
	if x < 0 || x >= g.W || y < 0 || y >= g.H {
		return nil
	}
	return g.Tiles[y*g.W+x]
}

func (g GameBoard) Set(x, y int, t *Tile) {
	if x < 0 || x >= g.W || y < 0 || y >= g.H {
		return
	}
	g.Tiles[y*g.W+x] = t
}

// Game implements ebiten.Game interface.
type Game struct {
	Board GameBoard

	HoveredTile         *Tile
	LeftClickTileBuffer *Tile
	IsChord             bool

	IsLost bool
	IsWin  bool

	IsShowDebug bool
}

func (g *Game) resetBoardPressState() {
	for _, t := range g.Board.Tiles {
		t.Unfocus()
	}
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		g.IsShowDebug = !g.IsShowDebug
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		g.newGame()
		return nil
	}

	if g.IsLost || g.IsWin {
		return nil
	}

	x, y := ebiten.CursorPosition()
	x, y = x/TileSize, y/TileSize
	g.HoveredTile = g.Board.At(x, y)

	if g.HoveredTile != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			g.resetBoardPressState()
			g.IsChord = true
			g.LeftClickTileBuffer = g.HoveredTile
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					current := g.Board.At(g.HoveredTile.X+i, g.HoveredTile.Y+j)
					if current == nil {
						continue
					}
					current.Focus()
				}
			}
		} else if g.IsChord {
			if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || !ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
				g.resetBoardPressState()
				g.IsChord = false
				if g.HoveredTile.IsOpened {
					counter := TileValue(0)
					for i := -1; i <= 1; i++ {
						for j := -1; j <= 1; j++ {
							if i == 0 && j == 0 {
								continue
							}
							current := g.Board.At(g.HoveredTile.X+i, g.HoveredTile.Y+j)
							if current == nil {
								continue
							}
							if current.IsFlagged {
								counter++
							}
						}
					}
					if counter == g.HoveredTile.Value {
						for i := -1; i <= 1; i++ {
							for j := -1; j <= 1; j++ {
								if i == 0 && j == 0 {
									continue
								}
								current := g.Board.At(g.HoveredTile.X+i, g.HoveredTile.Y+j)
								if current == nil || current.IsFlagged {
									continue
								}
								current.Open()
								if current.Value == TileValueEmpty {
									g.openAdjacent(current)
								} else if current.Value == TileValueMine {
									g.IsLost = true
								}
							}
						}
					}
				}
			}
		} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if g.LeftClickTileBuffer == nil || g.LeftClickTileBuffer != g.HoveredTile {
				g.resetBoardPressState()
				g.HoveredTile.Focus()
				g.LeftClickTileBuffer = g.HoveredTile
			}
		} else if g.LeftClickTileBuffer != nil && g.LeftClickTileBuffer.IsFocussed {
			g.resetBoardPressState()
			g.HoveredTile.Open()
			g.LeftClickTileBuffer = nil
			if g.HoveredTile.Value == TileValueEmpty {
				g.openAdjacent(g.HoveredTile)
			} else if g.HoveredTile.Value == TileValueMine {
				g.IsLost = true
			}
		} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			g.HoveredTile.ToggleFlag()
		}
	}

	g.checkWin()

	return nil
}

func (g *Game) checkWin() {
	if g.IsLost {
		return
	}
	for _, t := range g.Board.Tiles {
		if t.Value != TileValueMine && !t.IsOpened {
			return
		}
	}
	g.IsWin = true
}

func (g *Game) openAdjacent(t *Tile) {
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i == 0 && j == 0 {
				continue
			}
			x, y := t.X+i, t.Y+j
			next := g.Board.At(x, y)
			if next == nil || next.IsOpened {
				continue
			}
			next.Open()
			if next.Value == TileValueEmpty {
				g.openAdjacent(next)
			}
		}
	}
}

type drawBuffer struct {
	img *ebiten.Image
	opt *ebiten.DrawImageOptions
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	buf1 := make([]drawBuffer, 0, len(g.Board.Tiles))
	buf2 := make([]drawBuffer, 0, len(g.Board.Tiles))
	for _, t := range g.Board.Tiles {
		opts := t.DrawOpts
		if g.IsLost && g.HoveredTile == t {
			opts = &ebiten.DrawImageOptions{}
			opts.GeoM.Concat(t.DrawOpts.GeoM)
			opts.ColorM.Translate(0.3, 0, 0, 0)
		} else if g.IsWin {
			opts = &ebiten.DrawImageOptions{}
			opts.GeoM.Concat(t.DrawOpts.GeoM)
			opts.ColorM.Translate(0, 0.3, 0, 0)
		}
		if !t.IsOpened {
			buf1 = append(buf1, drawBuffer{t.DrawImg, opts})
		} else {
			buf2 = append(buf2, drawBuffer{t.DrawImg, opts})
		}
	}

	for _, d := range buf1 {
		screen.DrawImage(d.img, d.opt)
	}
	for _, d := range buf2 {
		screen.DrawImage(d.img, d.opt)
	}
	fface := mplusFont(TileSize - TileInsetSize*2)
	for _, t := range g.Board.Tiles {
		if t.DrawText == "" {
			continue
		}
		text.Draw(screen, t.DrawText, fface, t.DrawTextX, t.DrawTextY, color.Black)
	}

	if g.IsShowDebug {
		// Draw info
		fnt := mplusFont(10)
		msg := fmt.Sprintf("TPS: %0.2f, FPS: %0.2f", ebiten.CurrentTPS(), ebiten.CurrentFPS())
		text.Draw(screen, msg, fnt, 20, 20, color.White)
	}
}

// Layout takes the outside size (e.g., the window size) and returns the (logical) screen size.
// If you don't have to adjust the screen size with the outside size, just return a fixed size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
}

func (g *Game) newGame() {
	rand.Seed(time.Now().UnixNano())
	boardH, boardW := 24, 32
	board := GameBoard{Tiles: make([]*Tile, boardW*boardH), W: boardW, H: boardH}
	for x := 0; x < board.W; x++ {
		for y := 0; y < board.H; y++ {
			board.Set(x, y, NewTile(0, x, y))
		}
	}
	perms := rand.Perm(len(board.Tiles))
	for i := 0; i < 99; i++ {
		board.Tiles[perms[i]].Value = TileValueMine
	}
	for _, t := range board.Tiles {
		if t.Value == TileValueMine {
			continue
		}
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				if i == 0 && j == 0 {
					continue
				}
				x, y := t.X+i, t.Y+j
				if x < 0 || y < 0 || x >= board.W || y >= board.H {
					continue
				}
				if board.At(x, y).Value == TileValueMine {
					t.Value++
				}
			}
		}
	}
	g.Board = board
	g.HoveredTile = nil
	g.IsChord = false
	g.LeftClickTileBuffer = nil
	g.IsLost = false
	g.IsWin = false
}

type TileValue int

const (
	TileValueMine  = -1
	TileValueEmpty = 0
)

const (
	TileSize      = 16
	TileInsetSize = 1
)

var (
	emptyTileImageCached *ebiten.Image

	mplusFontCached map[float64]font.Face
)

func mplusFont(size float64) font.Face {
	if mplusFontCached == nil {
		mplusFontCached = map[float64]font.Face{}
	}
	fnt, ok := mplusFontCached[size]
	if !ok {
		tt, err := opentype.Parse(fonts.MPlus1pRegular_ttf)
		if err != nil {
			panic(err)
		}

		fnt, err = opentype.NewFace(tt, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			panic(err)
		}
		mplusFontCached[size] = fnt
	}
	return fnt
}

func emptyTileImage() *ebiten.Image {
	if emptyTileImageCached == nil {
		emptyTileImageCached = ebiten.NewImage(TileSize, TileSize)
		emptyTileImageCached.Fill(color.Gray{Y: 128})

		tileInnerImg := ebiten.NewImage(TileSize-TileInsetSize*2, TileSize-TileInsetSize*2)
		tileInnerImg.Fill(color.Gray{Y: 180})
		tileInnerImgOpts := &ebiten.DrawImageOptions{}
		tileInnerImgOpts.GeoM.Translate(TileInsetSize, TileInsetSize)

		emptyTileImageCached.DrawImage(tileInnerImg, tileInnerImgOpts)
	}
	return emptyTileImageCached
}

// Tile holds information of a single tile
type Tile struct {
	Value      TileValue
	X, Y       int
	IsOpened   bool
	IsFlagged  bool
	IsFocussed bool

	DrawX, DrawY float64
	DrawImg      *ebiten.Image
	DrawOpts     *ebiten.DrawImageOptions

	DrawTextX, DrawTextY int
	DrawText             string
}

// NewTile creates a new tile with the given value at given coordinates
func NewTile(v TileValue, x, y int) *Tile {
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(TileSize*x), float64(TileSize*y))
	return &Tile{
		Value: v,
		X:     x, Y: y,
		DrawX: float64(TileSize * x), DrawY: float64(TileSize * y),
		DrawImg:  emptyTileImage(),
		DrawOpts: opts,
	}
}

func (t *Tile) ValueString() string {
	switch t.Value {
	case TileValueMine:
		return "M"
	case TileValueEmpty:
		return ""
	default:
		return strconv.Itoa(int(t.Value))
	}
}

func (t *Tile) ToggleFlag() {
	if t.IsOpened {
		return
	}
	t.IsFlagged = !t.IsFlagged
	t.DrawText = ""
	if t.IsFlagged {
		t.DrawText = "F"
	}
	t.setupDrawTextPos()
}

func (t *Tile) setupDrawTextPos() {
	fface := mplusFont(TileSize - TileInsetSize*2)
	bounds := text.BoundString(fface, t.DrawText)
	t.DrawTextX = TileSize*t.X + (TileSize-bounds.Dx())/2
	t.DrawTextY = TileSize*t.Y + bounds.Dy() + TileInsetSize + (1 - bounds.Max.Y)
}

func (t *Tile) Open() {
	t.IsFlagged = false
	t.IsOpened = true
	t.DrawText = t.ValueString()
	t.setupDrawTextPos()
	t.DrawOpts.ColorM.Reset()
	t.DrawOpts.ColorM.Translate(-0.1, -0.1, -0.1, 0)
}

func (t *Tile) Unfocus() {
	if !t.IsFocussed {
		return
	}
	t.IsFocussed = false
	t.DrawOpts.ColorM.Reset()
}

func (t *Tile) Focus() {
	if t.IsFlagged || t.IsOpened {
		return
	}
	t.IsFocussed = true

	t.DrawOpts.ColorM.Reset()
	t.DrawOpts.ColorM.Translate(0.1, 0.1, 0, 0)
}

func main() {
	game := &Game{}
	game.newGame()
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Minesweeper Go")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
