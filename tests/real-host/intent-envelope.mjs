// Local model receipt labels vary; the original CLI intent remains unchanged.
export function intentFromReceipt(receipt){
 const keys=['intent','acceptIntent','cliIntent','intentCliJson','cliOutput','cliJson','intentCall','cliReturn'].filter(key=>receipt[key]!==undefined);
 if(keys.length>1)throw new Error('Ambiguous original CLI intent wrappers');
 const envelope=keys.length?receipt[keys[0]]:receipt;
 const outputs=['stdoutJson','stdout','cliStdout'].filter(key=>envelope?.[key]!==undefined);
 if(outputs.length&&envelope.exitCode!==0)throw new Error('Original CLI execution failed');
 if(outputs.length>1)throw new Error('Ambiguous original CLI output');
 const output=outputs.length?envelope[outputs[0]]:envelope;
 const raw=typeof output==='string'?JSON.parse(output):output;
 if(raw?.ok===false)throw new Error('The original CLI did not prepare an intent');
 const intent=raw?.data??raw;
 if(!/^[0-9a-f-]{36}$/.test(intent?.id??'')||!/^[0-9a-f-]{36}$/.test(intent?.objectId??'')||typeof intent?.confirmUrl!=='string')throw new Error('Complete original CLI intent required');
 return intent;
}
