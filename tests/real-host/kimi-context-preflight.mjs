// A smaller client window cannot fit history that native compaction preserves.
// Use observed numeric metrics only; never rewrite the native session history.
export function assessContextWindow(compactions, contextSize, outputSize) {
 const recent=compactions.filter(x=>Number.isSafeInteger(x.tokensAfter)&&x.tokensAfter>0).slice(-3);
 if(!recent.length)return {admitted:true,reason:'no native compaction measurements available',observations:0};
 const observedMinimum=Math.min(...recent.map(x=>x.tokensAfter));
 const requiredObservedMinimum=observedMinimum+outputSize;
 return {admitted:requiredObservedMinimum<contextSize,observations:recent.length,observedMinimum,outputSize,contextSize,requiredObservedMinimum,
  reason:requiredObservedMinimum<contextSize?'observed compacted context leaves declared output room':'native compacted context plus output allowance does not fit the declared client window'};
}
