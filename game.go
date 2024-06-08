package chess

import (
	"errors"
	"fmt"
	"io"
)

// A Outcome is the result of a game.
type Outcome string

const (
	// NoOutcome indicates that a game is in progress or ended without a result.
	NoOutcome Outcome = "*"
	// WhiteWon indicates that white won the game.
	WhiteWon Outcome = "1-0"
	// BlackWon indicates that black won the game.
	BlackWon Outcome = "0-1"
	// Draw indicates that game was a draw.
	Draw Outcome = "1/2-1/2"
)

// String implements the fmt.Stringer interface
func (o Outcome) String() string {
	return string(o)
}

// A Method is the method that generated the outcome.
type Method uint8

const (
	// NoMethod indicates that an outcome hasn't occurred or that the method can't be determined.
	NoMethod Method = iota
	// Checkmate indicates that the game was won checkmate.
	Checkmate
	// Resignation indicates that the game was won by resignation.
	Resignation
	// DrawOffer indicates that the game was drawn by a draw offer.
	DrawOffer
	// Stalemate indicates that the game was drawn by stalemate.
	Stalemate
	// ThreefoldRepetition indicates that the game was drawn when the game
	// state was repeated three times and a player requested a draw.
	ThreefoldRepetition
	// FivefoldRepetition indicates that the game was automatically drawn
	// by the game state being repeated five times.
	FivefoldRepetition
	// FiftyMoveRule indicates that the game was drawn by the half
	// move clock being one hundred or greater when a player requested a draw.
	FiftyMoveRule
	// SeventyFiveMoveRule indicates that the game was automatically drawn
	// when the half move clock was one hundred and fifty or greater.
	SeventyFiveMoveRule
	// InsufficientMaterial indicates that the game was automatically drawn
	// because there was insufficient material for checkmate.
	InsufficientMaterial
)

// TagPair represents metadata in a key value pairing used in the PGN format.
type TagPair struct {
	Key   string
	Value string
}

const MaxMoves = 600

// A Game represents a single chess game.
type Game struct {
	notation             Notation
	moves                [MaxMoves]*Move
	comments             [MaxMoves][]string
	positions            [MaxMoves]*Position
	currentMove          int
	ignoreAutomaticDraws bool
}

// PGN takes a reader and returns a function that updates
// the game to reflect the PGN data.  The PGN can use any
// move notation supported by this package.  The returned
// function is designed to be used in the NewGame constructor.
// An error is returned if there is a problem parsing the PGN data.
func PGN(r io.Reader) (func(*Game), error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	game, err := decodePGN(string(b))
	if err != nil {
		return nil, err
	}
	return func(g *Game) {
		g.copy(game)
	}, nil
}

// FEN takes a string and returns a function that updates
// the game to reflect the FEN data.  Since FEN doesn't encode
// prior moves, the move list will be empty.  The returned
// function is designed to be used in the NewGame constructor.
// An error is returned if there is a problem parsing the FEN data.
func FEN(fen string) (func(*Game), error) {
	pos, err := decodeFEN(fen)
	if err != nil {
		return nil, err
	}
	return func(g *Game) {
		pos.inCheck = isInCheck(pos)
		g.positions[g.currentMove] = pos
		g.updatePosition()
	}, nil
}

// UseNotation returns a function that sets the game's notation
// to the given value.  The notation is used to parse the
// string supplied to the MoveStr() method as well as the
// any PGN output.  The returned function is designed
// to be used in the NewGame constructor.
func UseNotation(n Notation) func(*Game) {
	return func(g *Game) {
		g.notation = n
	}
}

// NewGame defaults to returning a game in the standard
// opening position.  Options can be given to configure
// the game's initial state.
func NewGame(options ...func(*Game)) *Game {
	pos := StartingPosition()
	game := &Game{
		notation:    AlgebraicNotation{}, // Используйте вашу реализацию Notation
		currentMove: 0,
	}
	game.positions[0] = pos

	for _, f := range options {
		if f != nil {
			f(game)
		}
	}
	return game
}

// Move updates the game with the given move.  An error is returned
// if the move is invalid or the game has already been completed.
func (g *Game) Move(m *Move) error {
	valid := m // move is assumed to be valid and passed directly
	g.moves[g.currentMove+1] = valid
	pos := g.positions[g.currentMove].Update(valid)
	g.positions[g.currentMove+1] = pos
	g.currentMove += 1
	g.updatePosition()
	return nil
}

func (g *Game) UnMove() error {
	if g.currentMove > 0 {
		g.currentMove -= 1
	}
	return nil
}

// MoveStr decodes the given string in game's notation
// and calls the Move function.  An error is returned if
// the move can't be decoded or the move is invalid.
func (g *Game) MoveStr(s string) error {
	m, err := g.notation.Decode(g.positions[g.currentMove], s)
	if err != nil {
		return err
	}
	return g.Move(m)
}

// ValidMoves returns a list of valid moves in the
// current position.
func (g *Game) ValidMoves() []*Move {
	return g.positions[g.currentMove].ValidMoves()
}

// Positions returns the position history of the game.
func (g *Game) Positions() []*Position {
	return g.positions[:]
}

// Moves returns the move history of the game.
func (g *Game) Moves() []*Move {
	return g.moves[:]
}

// Comments returns the comments for the game indexed by moves.
func (g *Game) Comments() [][]string {
	return g.comments[:]
}

// Position returns the game's current position.
func (g *Game) Position() *Position {
	return g.positions[g.currentMove]
}

// Outcome returns the game outcome.
func (g *Game) Outcome() Outcome {
	return g.positions[g.currentMove].outcome
}

// Method returns the method in which the outcome occurred.
func (g *Game) Method() Method {
	return g.positions[g.currentMove].method
}

// FEN returns the FEN notation of the current position.
func (g *Game) FEN() string {
	return g.positions[g.currentMove].String()
}

// String implements the fmt.Stringer interface and returns
// the game's PGN.
func (g *Game) String() string {
	return encodePGN(g)
}

// MarshalText implements the encoding.TextMarshaler interface and
// encodes the game's PGN.
func (g *Game) MarshalText() (text []byte, err error) {
	return []byte(encodePGN(g)), nil
}

// UnmarshalText implements the encoding.TextUnarshaler interface and
// assumes the data is in the PGN format.
func (g *Game) UnmarshalText(text []byte) error {
	game, err := decodePGN(string(text))
	if err != nil {
		return err
	}
	g.copy(game)
	return nil
}

// Draw attempts to draw the game by the given method.  If the
// method is valid, then the game is updated to a draw by that
// method.  If the method isn't valid then an error is returned.
func (g *Game) Draw(method Method) error {
	switch method {
	case ThreefoldRepetition:
		if g.numOfRepetitions() < 3 {
			return errors.New("chess: draw by ThreefoldRepetition requires at least three repetitions of the current board state")
		}
	case FiftyMoveRule:
		if g.positions[g.currentMove].halfMoveClock < 100 {
			return fmt.Errorf("chess: draw by FiftyMoveRule requires the half move clock to be at 100 or greater but is %d", g.positions[g.currentMove].halfMoveClock)
		}
	case DrawOffer:
	default:
		return fmt.Errorf("chess: unsupported draw method %s", method.String())
	}
	g.positions[g.currentMove].outcome = Draw
	g.positions[g.currentMove].method = method
	return nil
}

// Resign resigns the game for the given color.  If the game has
// already been completed then the game is not updated.
func (g *Game) Resign(color Color) {
	if g.positions[g.currentMove].outcome != NoOutcome || color == NoColor {
		return
	}
	if color == White {
		g.positions[g.currentMove].outcome = BlackWon
	} else {
		g.positions[g.currentMove].outcome = WhiteWon
	}
	g.positions[g.currentMove].method = Resignation
}

// EligibleDraws returns valid inputs for the Draw() method.
func (g *Game) EligibleDraws() []Method {
	draws := []Method{DrawOffer}
	if g.numOfRepetitions() >= 3 {
		draws = append(draws, ThreefoldRepetition)
	}
	if g.positions[g.currentMove].halfMoveClock >= 100 {
		draws = append(draws, FiftyMoveRule)
	}
	return draws
}

// MoveHistory is a move's result from Game's MoveHistory method.
// It contains the move itself, any comments, and the pre and post
// positions.
type MoveHistory struct {
	PrePosition  *Position
	PostPosition *Position
	Move         *Move
	Comments     []string
}

// MoveHistory returns the moves in order along with the pre and post
// positions and any comments.
func (g *Game) MoveHistory() []*MoveHistory {
	h := []*MoveHistory{}
	for i, p := range g.positions {
		if i == 0 {
			continue
		}
		m := g.moves[i-1]
		c := g.comments[i-1]
		mh := &MoveHistory{
			PrePosition:  g.positions[i-1],
			PostPosition: p,
			Move:         m,
			Comments:     c,
		}
		h = append(h, mh)
	}
	return h
}

func (g *Game) updatePosition() {
	method := g.positions[g.currentMove].Status()
	if method == Stalemate {
		g.positions[g.currentMove].method = Stalemate
		g.positions[g.currentMove].outcome = Draw
	} else if method == Checkmate {
		g.positions[g.currentMove].method = Checkmate
		g.positions[g.currentMove].outcome = WhiteWon
		if g.positions[g.currentMove].Turn() == White {
			g.positions[g.currentMove].outcome = BlackWon
		}
	} else if method == NoMethod {
		g.positions[g.currentMove].method = NoMethod
		g.positions[g.currentMove].outcome = NoOutcome
	}

	if g.positions[g.currentMove].outcome != NoOutcome {
		return
	}

	// five fold rep creates automatic draw
	if !g.ignoreAutomaticDraws && g.numOfRepetitions() >= 5 {
		g.positions[g.currentMove].outcome = Draw
		g.positions[g.currentMove].method = FivefoldRepetition
	}

	// 75 move rule creates automatic draw
	if !g.ignoreAutomaticDraws && g.positions[g.currentMove].halfMoveClock >= 150 && g.positions[g.currentMove].method != Checkmate {
		g.positions[g.currentMove].outcome = Draw
		g.positions[g.currentMove].method = SeventyFiveMoveRule
	}

	// insufficient material creates automatic draw
	if !g.ignoreAutomaticDraws && !g.positions[g.currentMove].board.hasSufficientMaterial() {
		g.positions[g.currentMove].outcome = Draw
		g.positions[g.currentMove].method = InsufficientMaterial
	}
}

func (g *Game) copy(game *Game) {
	// Копируем moves
	for i := 0; i < MaxMoves; i++ {
		if game.moves[i] != nil {
			g.moves[i] = game.moves[i]
		} else {
			break
		}
	}

	// Копируем positions
	for i := 0; i < MaxMoves; i++ {
		if game.positions[i] != nil {
			g.positions[i] = game.positions[i]
		} else {
			break
		}
	}

	// Копируем comments
	for i := 0; i < MaxMoves; i++ {
		if game.comments[i] != nil {
			g.comments[i] = make([]string, len(game.comments[i]))
			copy(g.comments[i], game.comments[i])
		} else {
			break
		}
	}

	// Копируем текущее количество ходов
	g.currentMove = game.currentMove
}

func (g *Game) Clone() *Game {
	// Создаем новый экземпляр Game
	newGame := &Game{
		notation:    g.notation,
		currentMove: g.currentMove,
	}

	// Копируем moves
	for i := 0; i < len(g.moves); i++ {
		if g.moves[i] != nil {
			newGame.moves[i] = g.moves[i]
		} else {
			break
		}
	}

	// Копируем positions
	for i := 0; i < len(g.positions); i++ {
		if g.positions[i] != nil {
			newGame.positions[i] = g.positions[i]
		} else {
			break
		}
	}

	// Копируем comments
	for i := 0; i < len(g.comments); i++ {
		if g.comments[i] != nil {
			newGame.comments[i] = make([]string, len(g.comments[i]))
			copy(newGame.comments[i], g.comments[i])
		} else {
			break
		}
	}

	return newGame
}

func (g *Game) numOfRepetitions() int {
	count := 0
	for _, pos := range g.Positions() {
		if g.positions[g.currentMove].samePosition(pos) {
			count++
		}
	}
	return count
}
