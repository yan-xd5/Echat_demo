package filter
import ("context";"fmt";"strings";"trpc.group/trpc-go/trpc-go";"trpc.group/trpc-go/trpc-go/filter";thttp"trpc.group/trpc-go/trpc-go/http";"echat/sdk/usecase/auth";"echat/service/api/internal/shared")
func init(){filter.Register("apiAuth",serverFilter,nil)}
func skipAuth(n string)bool{for _,s:=range[]string{"/Register","/Login","/GetPublicKey"}{if strings.HasSuffix(n,s){return true}};return false}
func extractToken(ctx context.Context,md map[string][]byte)string{
	if md!=nil{for k,v:=range md{if strings.EqualFold(k,"authorization"){return strings.TrimPrefix(string(v),"Bearer ")}}}
	if r:=thttp.Request(ctx);r!=nil{if a:=r.Header.Get("Authorization");a!=""{return strings.TrimPrefix(a,"Bearer ")}}
	return""
}
func serverFilter(ctx context.Context,req interface{},next filter.ServerHandleFunc)(interface{},error){
	msg:=trpc.Message(ctx);if msg==nil{return next(ctx,req)}
	n:=msg.ServerRPCName();var uid string
	if t:=extractToken(ctx,msg.ServerMetaData());t!=""{
		if c,e:=auth.ParseToken(t);e==nil{uid=c.UID;ctx=context.WithValue(ctx,shared.UIDKey,uid)}else if!skipAuth(n){return nil,fmt.Errorf("token无效")}
	}
	if!skipAuth(n)&&uid==""{return nil,fmt.Errorf("未认证")}
	return next(ctx,req)
}
