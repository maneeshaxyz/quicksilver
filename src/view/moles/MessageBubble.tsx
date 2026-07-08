import { useEffect, useRef, useState } from "react";
import { Box } from "@mui/material";
import MessageContent from "../atoms/MessageContent";
import MessageMeta from "../atoms/MessageMeta";
import AttachmentList from "./AttachmentList";
import AttachmentViewer from "./AttachmentViewer";

const MessageBubble = ({
  message,
  isSent,
  onDownloadAttachment,
  onFetchAttachment,
}) => {
  // Full HTML email bodies are documents, not chat messages — they need the
  // whole column width. Plain-text replies stay in the narrow chat-bubble look.
  const wide = !!message.contentHtml;

  // Map the server attachment DTO (id/filename/mime_type/size) onto the shape
  // AttachmentPreview/AttachmentInfo expect (id/name/size).
  const attachments = (message.attachments || []).map((a) => ({
    id: a.id,
    name: a.filename,
    size: a.size,
    mimeType: a.mime_type,
  }));

  // In-app preview dialog state. The blob object URL is created here on open
  // and revoked on close/unmount so we never leak memory across previews.
  const [preview, setPreview] = useState(null);
  const urlRef = useRef(null);

  const revoke = () => {
    if (urlRef.current) {
      URL.revokeObjectURL(urlRef.current);
      urlRef.current = null;
    }
  };
  useEffect(() => revoke, []); // cleanup on unmount

  const handlePreview = async (att) => {
    if (!onFetchAttachment) return;
    revoke();
    setPreview({
      id: att.id,
      name: att.name,
      mimeType: att.mimeType,
      url: null,
      loading: true,
      error: null,
    });
    try {
      const blob = await onFetchAttachment(message.sourceThreadId, att.id);
      const url = URL.createObjectURL(blob);
      urlRef.current = url;
      setPreview((p) =>
        p ? { ...p, url, loading: false } : p,
      );
    } catch (e) {
      setPreview((p) =>
        p
          ? { ...p, loading: false, error: e?.message || "Preview failed" }
          : p,
      );
    }
  };

  const closePreview = () => {
    revoke();
    setPreview(null);
  };

  return (
    <Box
      sx={{
        alignSelf: wide ? "stretch" : isSent ? "flex-end" : "flex-start",
        backgroundColor: "background.paper",
        color: "text.primary",
        borderRadius: 2,
        px: 2,
        py: 1.5,
        width: wide ? "100%" : "auto",
        maxWidth: wide ? "100%" : "75%",
        minWidth: 0,
        boxShadow: 1,
      }}
    >
      <MessageContent
        content={message.content}
        contentHtml={message.contentHtml}
      />
      <AttachmentList
        attachments={attachments}
        editable={false}
        onDownload={
          onDownloadAttachment
            ? (att) => onDownloadAttachment(message.sourceThreadId, att.id, att.name)
            : undefined
        }
        onPreview={onFetchAttachment ? handlePreview : undefined}
      />
      <MessageMeta
        timestamp={message.timestamp}
        isRead={message.isRead}
        isSent={isSent}
      />
      <AttachmentViewer
        open={!!preview}
        onClose={closePreview}
        url={preview?.url}
        mimeType={preview?.mimeType || ""}
        filename={preview?.name || "attachment"}
        loading={!!preview?.loading}
        error={preview?.error}
        onDownload={
          onDownloadAttachment && preview
            ? () => onDownloadAttachment(message.sourceThreadId, preview.id, preview.name)
            : null
        }
      />
    </Box>
  );
};

export default MessageBubble;
