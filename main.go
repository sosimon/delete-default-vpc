package main

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
)

// https://docs.aws.amazon.com/sdk-for-go/api/service/sts/#STS.GetCallerIdentity
func getCallerIdentity(session client.ConfigProvider) error {
	stsSvc := sts.New(session)
	result, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		handleError(err)
	}
	if result.Arn != nil {
		fmt.Printf("Logged in as: %s\n", *result.Arn)
	}
	return err
}

func getRegions(svc *ec2.EC2) []string {
	result, err := svc.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		handleError(err)
		return nil
	}

	if result != nil {
		output := []string{}
		for _, region := range result.Regions {
			output = append(output, *region.RegionName)
		}
		return output
	}

	fmt.Println("Error: no regions found")
	return nil
}

func deleteDefaultVpc(wg *sync.WaitGroup, sess client.ConfigProvider, region string) {
	defer wg.Done()
	svc := ec2.New(sess, aws.NewConfig().WithRegion(region))
	result, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []*string{aws.String("true")},
			},
		},
	})
	if err != nil {
		handleError(err)
		return
	}
	if result != nil && len(result.Vpcs) > 0 {
		vpcID := *result.Vpcs[0].VpcId
		fmt.Printf("[%s] Found default VPC: %v\n", region, vpcID)
		deleteInternetGateway(svc, region, vpcID)
		deleteSubnets(svc, region, vpcID)
		deleteVpc(svc, region, vpcID)
	} else {
		fmt.Printf("[%s] No default VPC found\n", region)
	}
}

func deleteInternetGateway(svc *ec2.EC2, region string, vpcID string) {
	result, err := svc.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("attachment.vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	})
	if err != nil {
		handleError(err)
		return
	}
	if result != nil && len(result.InternetGateways) > 0 {
		igw := *result.InternetGateways[0].InternetGatewayId

		fmt.Printf("[%s] Detaching and deleting internet gateway: %v\n", region, igw)

		_, err := svc.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(igw),
			VpcId:             aws.String(vpcID),
		})
		if err != nil {
			handleError(err)
			return
		}

		_, err = svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(igw),
		})
		if err != nil {
			handleError(err)
			return
		}
	} else {
		fmt.Printf("[%s] No internet gateway found for vpc %s\n", region, vpcID)
	}
}

func deleteSubnets(svc *ec2.EC2, region string, vpcID string) {
	result, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	})
	if err != nil {
		handleError(err)
		return
	}
	if result != nil && len(result.Subnets) > 0 {
		subnets := result.Subnets
		for _, subnet := range subnets {
			s := *subnet.SubnetId
			fmt.Printf("[%s] Deleting subnet: %v\n", region, s)
			_, err := svc.DeleteSubnet(&ec2.DeleteSubnetInput{
				SubnetId: aws.String(s),
			})
			if err != nil {
				handleError(err)
				return
			}
		}
	} else {
		fmt.Printf("[%s] No subnets found for vpc %s\n", region, vpcID)
	}
}

func deleteVpc(svc *ec2.EC2, region string, vpcID string) {
	fmt.Printf("[%s] Deleting VPC: %s\n", region, vpcID)
	_, err := svc.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		handleError(err)
		return
	}
}

func handleError(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		default:
			fmt.Println(aerr.Error())
		}
	} else {
		fmt.Println(err.Error())
	}
}

func main() {
	// Creat new AWS Session
	sess, err := session.NewSession()
	if err != nil {
		fmt.Printf("Error creating session: %s\n", err)
		return
	}

	// Display Account, Role, ARN information
	err = getCallerIdentity(sess)
	if err != nil {
		fmt.Printf("Error getting caller identity: %s\n", err)
		return
	}

	svc := ec2.New(sess, aws.NewConfig().WithRegion("us-east-1"))

	regions := getRegions(svc)
	fmt.Printf("Regions: %v\n", regions)

	r := make(chan string)
	go func() {
		for _, region := range regions {
			r <- region
		}
		close(r)
	}()

	var wg sync.WaitGroup
	for region := range r {
		wg.Add(1)
		go deleteDefaultVpc(&wg, sess, region)
	}
	wg.Wait()
}
