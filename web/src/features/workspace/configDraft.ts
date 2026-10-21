import {useState} from 'react';

// A remote refresh never advances the version of a locally edited draft.
// Only an explicit reload or a successful save establishes a new baseline.
export function useConfigDraft<T>(latest:T,version:number){
 const [state,setState]=useState(()=>({draft:latest,baseVersion:version}));
 return {
  ...state,
  stale:version>state.baseVersion,
  update:(patch:Partial<T>|((draft:T)=>Partial<T>))=>setState(s=>({...s,draft:{...s.draft,...(typeof patch==='function'?patch(s.draft):patch)}})),
  replace:(draft:T,baseVersion:number)=>setState({draft,baseVersion}),
  reload:()=>setState({draft:latest,baseVersion:version}),
 };
}
