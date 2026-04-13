package kinopoisk

type Movie struct {
	ID               int            `json:"id"`
	Name             string         `json:"name"`
	AlternativeName  string         `json:"alternativeName"`
	EnName           string         `json:"enName"`
	Type             string         `json:"type"`
	Year             int            `json:"year"`
	Description      string         `json:"description"`
	ShortDescription string         `json:"shortDescription"`
	Slogan           string         `json:"slogan"`
	Status           string         `json:"status"`
	MovieLength      int            `json:"movieLength"`
	AgeRating        int            `json:"ageRating"`
	RatingMpaa       string         `json:"ratingMpaa"`
	IsSeries         bool           `json:"isSeries"`
	TicketsOnSale    bool           `json:"ticketsOnSale"`
	Top10            *int           `json:"top10"`
	Top250           *int           `json:"top250"`
	Rating           *Rating        `json:"rating"`
	Votes            *Votes         `json:"votes"`
	Poster           *Image         `json:"poster"`
	Backdrop         *Image         `json:"backdrop"`
	Logo             *Logo          `json:"logo"`
	Videos           *Videos        `json:"videos"`
	Genres           []ItemName     `json:"genres"`
	Countries        []ItemName     `json:"countries"`
	Persons          []Person       `json:"persons"`
	SimilarMovies    []LinkedMovie  `json:"similarMovies"`
	Budget           *CurrencyValue `json:"budget"`
	Fees             *Fees          `json:"fees"`
	Premiere         *Premiere      `json:"premiere"`
}

type Rating struct {
	KP                 float64 `json:"kp"`
	IMDB               float64 `json:"imdb"`
	TMDB               float64 `json:"tmdb"`
	FilmCritics        float64 `json:"filmCritics"`
	RussianFilmCritics float64 `json:"russianFilmCritics"`
}

type Votes struct {
	KP                 int `json:"kp"`
	IMDB               int `json:"imdb"`
	TMDB               int `json:"tmdb"`
	FilmCritics        int `json:"filmCritics"`
	RussianFilmCritics int `json:"russianFilmCritics"`
}

type Image struct {
	URL        string `json:"url"`
	PreviewURL string `json:"previewUrl"`
}

type Logo struct {
	URL string `json:"url"`
}

type Videos struct {
	Trailers []Video `json:"trailers"`
}

type Video struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Site string `json:"site"`
	Type string `json:"type"`
}

type ItemName struct {
	Name string `json:"name"`
}

type Person struct {
	ID           int    `json:"id"`
	Photo        string `json:"photo"`
	Name         string `json:"name"`
	EnName       string `json:"enName"`
	Description  string `json:"description"`
	Profession   string `json:"profession"`
	EnProfession string `json:"enProfession"`
}

type LinkedMovie struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	EnName          string  `json:"enName"`
	AlternativeName string  `json:"alternativeName"`
	Type            string  `json:"type"`
	Year            int     `json:"year"`
	Poster          *Image  `json:"poster"`
	Rating          *Rating `json:"rating"`
}

type CurrencyValue struct {
	Value    int    `json:"value"`
	Currency string `json:"currency"`
}

type Fees struct {
	World  *CurrencyValue `json:"world"`
	USA    *CurrencyValue `json:"usa"`
	Russia *CurrencyValue `json:"russia"`
}

type Premiere struct {
	World   string `json:"world"`
	USA     string `json:"usa"`
	Russia  string `json:"russia"`
	Digital string `json:"digital"`
	Cinema  string `json:"cinema"`
}

// SearchResponse - paginated response from /v1.4/movie/search
type SearchResponse struct {
	Docs  []Movie `json:"docs"`
	Total int     `json:"total"`
	Limit int     `json:"limit"`
	Page  int     `json:"page"`
	Pages int     `json:"pages"`
}

// CursorResponse - cursor-paginated response from /v1.5/movie
type CursorResponse struct {
	Docs    []Movie `json:"docs"`
	Limit   int     `json:"limit"`
	HasNext bool    `json:"hasNext"`
	HasPrev bool    `json:"hasPrev"`
	Next    string  `json:"next"`
	Prev    string  `json:"prev"`
	Total   *int    `json:"total"`
}
