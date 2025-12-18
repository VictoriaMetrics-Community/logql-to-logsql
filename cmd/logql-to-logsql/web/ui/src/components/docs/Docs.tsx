import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card.tsx";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion.tsx";
import { InfoIcon, LinkIcon } from "lucide-react";
import { Button } from "@/components/ui/button.tsx";
import { Separator } from "@/components/ui/separator.tsx";
import { Badge } from "@/components/ui/badge.tsx";

export function Docs() {
  return (
    <Card className={"w-full max-h-full min-w-[20rem] overflow-y-scroll py-4 border-none shadow-none drop-shadow-none"}>
      <CardHeader>
        <CardTitle className={"flex flex-row gap-2 items-center"}>
          <span>Information about LogQL to LogsQL</span>
          <Badge variant={"outline"}>
            {__APP_VERSION__ == "${VERSION}" ? "local" : __APP_VERSION__}
          </Badge>
        </CardTitle>
        <CardDescription>
          Service that helps to query VictoriaLogs with Loki LogQL
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className={"flex gap-2 items-center"}>
          <a
            href={"https://github.com/VictoriaMetrics-Community/logql-to-logsql"}
            target={"_blank"}
          >
            <Button variant={"link"} className={"cursor-pointer"}>
              <LinkIcon />
              Source code and documentation
            </Button>
          </a>
        </div>
        <Separator className={"mt-2 ml-3"} />
        <Accordion type="single" collapsible className="w-full pl-3">
          <AccordionItem value="statement-types">
            <AccordionTrigger className={"cursor-pointer"}>
              <span className={"flex flex-row gap-2 items-center"}>
                <InfoIcon size={16} />
                <span>Supported query types</span>
              </span>
            </AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <p>
                <ul className={"list-disc pl-4 pt-2"}>
                  <li>
                    <code>Log queries</code>
                  </li>
                  <li>
                    <code>Stats queries</code>
                  </li>
                </ul>
              </p>
            </AccordionContent>
          </AccordionItem>
          <AccordionItem value="clauses">
            <AccordionTrigger className={"cursor-pointer"}>
              <span className={"flex flex-row gap-2 items-center"}>
                <InfoIcon size={16} />
                <span>Supported pipeline stages</span>
              </span>
            </AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <p>
                <ul className={"list-disc pl-4 pt-2"}>
                  <li>
                    <code>|=, !=, |~, !~</code>
                  </li>
                  <li>
                    <code>| json, | logfmt, | regexp, | pattern</code>
                  </li>
                  <li>
                    <code>| drop, | keep</code>
                  </li>
                  <li>
                    <code>| line_format, | label_format</code>
                  </li>
                </ul>
              </p>
            </AccordionContent>
          </AccordionItem>
          <AccordionItem value="functions">
            <AccordionTrigger className={"cursor-pointer"}>
              <span className={"flex flex-row gap-2 items-center"}>
                <InfoIcon size={16} />
                <span>Supported metric functions (subset)</span>
              </span>
            </AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <p>
                <ul className={"list-disc pl-4 pt-2"}>
                  <li>
                    <code>rate, count_over_time</code>
                  </li>
                  <li>
                    <code>sum, topk, bottomk</code>
                  </li>
                  <li>
                    <code>avg_over_time, min_over_time, max_over_time</code>
                  </li>
                  <li>
                    <code>quantile_over_time</code>
                  </li>
                </ul>
              </p>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </CardContent>
    </Card>
  );
}
