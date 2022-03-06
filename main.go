package main;

import (
    // http://p.agnihotry.com/post/understanding_the_context_package_in_golang/
    "encoding/json"
    "io/ioutil"
    "fmt"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/user"
    "path/filepath"
    "flag"

    "golang.org/x/net/context"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/youtube/v3"
)

/// definitions
type Options struct {
    account_name string
    parts []string
}

type YoutubeChannel struct {
    identity string
    title string
    views uint64
}

type YoutubeVideo struct {
    identity string
    title string
}

type YoutubePlaylist struct {
    identity string
    channel YoutubeChannel
    videos []YoutubeVideo
}


const missingClientSecretsMessage = "Please configure Oauth 2.0"

/// What is the difference between an Arry and a Slice?
/// Slice can havy any number of entries while an array is set to an index
/// var slice_example []YoutubeAccount;
/// var array_example [1]YoutubeAccount;
/// https://stackoverflow.com/questions/13137463/declare-a-constant-array

/// Youtube QuickStart Functions
/// https://developers.google.com/youtube/v3/quickstart/go


// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
    cacheFile, err := tokenCacheFile()
    if err != nil {
        log.Fatalln("Unable to get path to cached credential file.")
    }

    tok, err := tokenFromFile(cacheFile)
    if err != nil {
        tok = getTokenFromWeb(config)
        saveToken(cacheFile, tok)
    }
    return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
    authUrl := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    fmt.Printf("Go to the following link in your browser, then type the "+
    "authorization code: \n%v\n", authUrl)
    var code string
    if _, err := fmt.Scan(&code); err != nil {
        log.Fatalf("Unable to read authorization code %v", err)
    }

    tok, err := config.Exchange(oauth2.NoContext, code)
    if err != nil {
        log.Fatalf("Unable to retrieve token from web %v", err)
    }
    return tok
}
// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
    usr, err := user.Current()
    if err != nil {
        return "", err
    }
    tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
    os.MkdirAll(tokenCacheDir, 0700)
    return filepath.Join(tokenCacheDir, url.QueryEscape("youtube-go-quickstart.json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
    f, err := os.Open(file)
    if err != nil {
        return nil, err
    }
    t := &oauth2.Token{}
    err = json.NewDecoder(f).Decode(t)
    // https://go.dev/tour/flowcontrol/12
    defer f.Close()
    return t, err
}
func saveToken(file string, token *oauth2.Token) {
    fmt.Printf("Saving credential file to: %s\n", file)
    f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        log.Fatalf("Unabel to cache oauth token: %v", err)
    }
    defer f.Close()
    json.NewEncoder(f).Encode(token)
}
func handleError(err error, message string) {
    if message == "" {
        message = "Erorr making API call"
    }
    if err != nil {
        log.Fatalf(message +": %v", err.Error())
    }
}

// func channelsListByUsername(service *youtube.Service, parts []string, forUsername string) {
func extractChannelListByUsername(service *youtube.Service, options Options) []YoutubeChannel {
    call := service.Channels.List(options.parts)
    call = call.ForUsername(options.account_name)
    response, err := call.Do()
    handleError(err, "")

    var channels []YoutubeChannel
    for _, entry := range response.Items {
        var channel = YoutubeChannel{
            entry.Id,
            entry.Snippet.Title,
            entry.Statistics.ViewCount,
        }
        channels = append(channels, channel)
    }
    return channels
}
func extractVideosByPlaylistIdentity(identity string, service *youtube.Service) []YoutubeVideo {
    var parts = []string{"id,snippet"}
    call := service.PlaylistItems.List(parts)
    call = call.PlaylistId(identity)
    response, err := call.Do()
    handleError(err, "")

    var videos []YoutubeVideo
    for _, entry := range response.Items {
        var video = YoutubeVideo{entry.Id, entry.Snippet.Title}
        videos = append(videos, video)
    }
    return videos
}
func extractPlaylistByChannel(channel YoutubeChannel, service *youtube.Service) []YoutubePlaylist {
    var parts = []string{"contentDetails,id,snippet"}
    call := service.Playlists.List(parts)
    call = call.ChannelId(channel.identity)
    response, err := call.Do()
    handleError(err, "")

    var playlists []YoutubePlaylist
    for _, entry := range response.Items {
        var playlist = YoutubePlaylist{
            entry.Id,
            channel,
            extractVideosByPlaylistIdentity(entry.Id, service),
        }
        playlists = append(playlists, playlist)
    }
    return playlists
}
func captureOptions() Options {
    account_name := flag.String("account-name", "foo", "Youtube Account Name")
    flag.Parse()
    return Options{
        *account_name,
        []string{"snippet,contentDetails,statistics"},
    }
}
func main() {
    // Setup Inputs/Outputs
    ctx := context.Background()
    options := captureOptions()
    fmt.Println("Looking Up account: ", options.account_name)


    // Setup Youtube
    b, err := ioutil.ReadFile("client_service.json")
    if err != nil {
        log.Fatalf("Unable to read client secret file: %v", err)
    }

    // if modifying these scopes, delete your previously saved credentials
    // at ~/.credentials/youtube-go-quickstart.json
    config, err := google.ConfigFromJSON(b, youtube.YoutubeReadonlyScope)
    if err != nil {
        log.Fatalf("Unable to parse client secret file to config: %v", err)
    }
    client := getClient(ctx, config)
    service, err := youtube.New(client)
    handleError(err, "Error creating YouTube client")

    // What is the difference between = and :=
    // https://stackoverflow.com/a/17891297/432309
    var channels = extractChannelListByUsername(service, options)
    for _, channel := range channels {
        fmt.Println(
            fmt.Sprintf("This channel's ID is %s. Its title is '%s', and it has %d views.",
                channel.identity,
                channel.title,
                channel.views))
        fmt.Println("Video Collection:")
        for _, playlist := range extractPlaylistByChannel(channel, service) {
            for _, video := range playlist.videos {
                fmt.Println(
                    fmt.Sprintf("Video ID is %s. Its title is '%s'.",
                    video.identity,
                    video.title))
            }
        }
    }
}
