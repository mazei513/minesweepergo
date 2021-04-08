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
	return g.Tiles[y*g.W+x]
}

// Game implements ebiten.Game interface.
type Game struct {
	Board GameBoard

	HoveredTile         *Tile
	LeftClickTileBuffer *Tile
	IsChord             bool

	IsLost bool

	IsShowDebug bool
}

func (g *Game) resetBoardPressState() {
	for _, t := range g.Board.Tiles {
		t.IsFocussed = false
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

	if g.IsLost {
		return nil
	}

	x, y := ebiten.CursorPosition()
	x, y = x/TileSize, y/TileSize
	if x >= 0 && x < g.Board.W && y >= 0 && y < g.Board.H {
		g.HoveredTile = g.Board.At(x, y)
	} else {
		g.HoveredTile = nil
	}

	if g.HoveredTile != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			g.resetBoardPressState()
			g.IsChord = true
			g.LeftClickTileBuffer = g.HoveredTile
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					x, y := g.HoveredTile.X+i, g.HoveredTile.Y+j
					if x < 0 || x >= g.Board.W || y < 0 || y >= g.Board.H {
						continue
					}
					current := g.Board.At(x, y)
					current.IsFocussed = !current.IsOpened
				}
			}
		} else if g.IsChord {
			if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || !ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
				g.resetBoardPressState()
				g.IsChord = false
			}
		} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if g.LeftClickTileBuffer == nil || g.LeftClickTileBuffer != g.HoveredTile {
				g.resetBoardPressState()
				g.HoveredTile.IsFocussed = !g.HoveredTile.IsOpened && !g.HoveredTile.IsFlagged
				g.LeftClickTileBuffer = g.HoveredTile
			}
		} else if g.LeftClickTileBuffer != nil && g.LeftClickTileBuffer.IsFocussed {
			g.resetBoardPressState()
			g.HoveredTile.IsOpened = true
			g.LeftClickTileBuffer = nil
			if g.HoveredTile.Value == TileValueEmpty {
				g.openAdjacent(g.HoveredTile)
			} else if g.HoveredTile.Value == TileValueMine {
				g.IsLost = true
			}
		} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			g.HoveredTile.IsFlagged = !g.HoveredTile.IsFlagged
		}
	}

	return nil
}

func (g *Game) openAdjacent(t *Tile) {
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i == 0 && j == 0 {
				continue
			}
			x, y := t.X+i, t.Y+j
			if x < 0 || y < 0 || x >= g.Board.W || y >= g.Board.H {
				continue
			}
			next := g.Board.At(x, y)
			if next.IsOpened {
				continue
			}
			next.IsOpened = true
			if next.Value == TileValueEmpty {
				g.openAdjacent(next)
			}
		}
	}
}

type drawBuffer struct {
	img          *ebiten.Image
	opt          *ebiten.DrawImageOptions
	text         string
	textX, textY int
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	buf := make([]drawBuffer, 0, len(g.Board.Tiles))
	for _, t := range g.Board.Tiles {
		img := emptyTileImage()
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(t.drawPosX(), t.drawPosY())
		if t.IsOpened {
			opts.ColorM.Translate(-0.1, -0.1, -0.1, 0)
		}
		cr, cg, cb := 0.0, 0.0, 0.0
		if g.IsLost && g.HoveredTile == t {
			cr = 0.3
		} else if t.IsFocussed {
			cr, cg = 0.1, 0.1
		}
		opts.ColorM.Translate(cr, cg, cb, 0)

		txt := ""
		if t.IsOpened {
			txt = t.ValueString()
		} else if t.IsFlagged {
			txt = "F"
		}
		buf = append(buf, drawBuffer{img, opts, txt, int(t.drawPosX()), int(t.drawPosY())})
	}

	for _, d := range buf {
		screen.DrawImage(d.img, d.opt)
	}
	fface, err := mplusFont(TileSize - TileInsetSize*2)
	if err != nil {
		panic(err)
	}
	for _, d := range buf {
		bounds := text.BoundString(fface, d.text)
		text.Draw(screen, d.text, fface, d.textX+(TileSize-bounds.Dx())/2, d.textY+bounds.Dy()+TileInsetSize+(1-bounds.Max.Y), color.Black)
	}

	if g.IsShowDebug {
		// Draw info
		fnt, err := mplusFont(10)
		if err != nil {
			panic(err)
		}
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
	for i := 0; i < len(board.Tiles); i++ {
		board.Tiles[i] = NewTile(0, 0, 0)
	}
	for x := 0; x < board.W; x++ {
		for y := 0; y < board.H; y++ {
			t := board.At(x, y)
			t.X, t.Y = x, y
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
}

type TileValue int

const (
	TileValueMine  = -1
	TileValueEmpty = 0
)

const (
	TileSize      = 16
	TileInsetSize = 2
)

var (
	emptyTileImageCached *ebiten.Image

	mplusFontCached map[float64]font.Face
)

func mplusFont(size float64) (font.Face, error) {
	if mplusFontCached == nil {
		mplusFontCached = map[float64]font.Face{}
	}
	fnt, ok := mplusFontCached[size]
	if !ok {
		tt, err := opentype.Parse(fonts.MPlus1pRegular_ttf)
		if err != nil {
			return nil, err
		}

		fnt, err = opentype.NewFace(tt, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, err
		}
		mplusFontCached[size] = fnt
	}
	return fnt, nil
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
}

// NewTile creates a new tile with the given value at given coordinates
func NewTile(v TileValue, x, y int) *Tile {
	return &Tile{Value: v, X: x, Y: y}
}

func (t *Tile) drawPosX() float64 { return float64(TileSize * t.X) }
func (t *Tile) drawPosY() float64 { return float64(TileSize * t.Y) }
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

func main() {
	game := &Game{}
	game.newGame()
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Minesweeper Go")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
