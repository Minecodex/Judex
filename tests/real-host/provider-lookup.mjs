import dns from 'node:dns';
import net from 'node:net';

export function providerLookup(host,address,fallback=dns.lookup){
 if(net.isIP(address)!==4)throw new Error('Test provider DNS pin requires a literal IPv4 address');
 return (hostname,options,callback)=>{
  if(hostname!==host)return fallback(hostname,options,callback);
  if(options?.all)return callback(null,[{address,family:4}]);
  return callback(null,address,4);
 };
}
