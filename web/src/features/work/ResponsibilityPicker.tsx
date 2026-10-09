import {UIOption,UISelect} from "../../components/ui/FormControls";
import {useWork} from "./store";

export function ResponsibilityPicker({ids,value,onChange,testId,label,showPerson=false}:{ids:string[];value:string;onChange:(id:string)=>void;testId:string;label?:string;showPerson?:boolean}){
 const {state,t,text}=useWork(),caption=label??t("workChooseSeat");
 return <UISelect aria-label={caption} data-testid={testId} value={value} onChange={e=>onChange(e.target.value)}><UIOption value="">{caption}</UIOption>{ids.map(id=>{
  const seat=state.seats.find(s=>s.id===id),position=state.positions.find(p=>p.id===seat?.positionId),name=text(position?.name??"");
  return <UIOption key={id} value={id}>{showPerson?`${seat?.person??""} · ${name}`:name}</UIOption>;
 })}</UISelect>;
}
