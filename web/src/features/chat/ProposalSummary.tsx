import {useState} from "react";
import {Card} from "@heroui/react";
import {GitBranch} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {useWork} from "../work/store";
import {ProposalReviewDialog} from "../cooperation/FrozenReview";
import type {WorkProposal} from "./proposalModel";
export function ProposalSummary({proposal}:{proposal:WorkProposal}){
 const {t,text}=useWork(),[review,setReview]=useState(false);
 return <><Card className="judex-co-summary" data-testid={"proposal-"+proposal.id}><Card.Header><div className="judex-co-card-top"><span className="judex-co-meta"><GitBranch/>{t("chatProposal")} · v{proposal.revision}</span><span className="judex-co-meta">{t(proposal.status==="approved"?"chatAccepted":proposal.status==="rejected"?"chatRejected":"chatWaiting")}</span></div><Card.Title>{text(proposal.title)}</Card.Title><Card.Description>{text(proposal.goal)}</Card.Description></Card.Header><Card.Footer><Button size="sm" variant="outline" onPress={()=>setReview(true)}>{t("coopReview")}</Button></Card.Footer></Card>{review&&<ProposalReviewDialog id={proposal.id} onClose={()=>setReview(false)}/>}</>;
}
