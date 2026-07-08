package shared
import ("context";"encoding/json";"net/http";"os";thttp "trpc.group/trpc-go/trpc-go/http";"echat/sdk/infrastructure/observability")
type UIDKeyType struct{}
var UIDKey = UIDKeyType{}
func GetUID(ctx context.Context) string {
	if v,ok:=ctx.Value(UIDKey).(string);ok{return v}
	if v:=thttp.Request(ctx).Header.Get("X-User-ID");v!=""{return v}
	return ""
}
var CorsOrigin=func()string{if v:=os.Getenv("CORS_ORIGIN");v!=""{return v};return"http://localhost:3000"}()
func WriteJSON(ctx context.Context,w http.ResponseWriter,code int,data interface{}){
	if w==nil{return}
	observability.SetHTTPStatus(ctx,code)
	w.Header().Set("Content-Type","application/json")
	w.Header().Set("Access-Control-Allow-Origin",CorsOrigin)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
func MsgError(s string)map[string]interface{}{return map[string]interface{}{"code":1,"message":s}}
func MsgSuccess(s string)map[string]interface{}{return map[string]interface{}{"code":0,"message":s}}
